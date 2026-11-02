import { fetchWithBackoff, isAbortError } from "@lib/backoff";

export const getHardwareConcurrency = () =>
  navigator.hardwareConcurrency !== undefined
    ? navigator.hardwareConcurrency
    : 1;

export type ProgressCallback = (nonce: number | string, scale: number) => void;

export interface ProcessOptions {
  basePrefix: string;
  version: string;
  algorithm: string;
}

export interface ProcessResult {
  hash: string;
  // The WASM workers don't echo the challenge data back, only the sha256
  // workers do. Nothing reads it, so it's optional rather than a lie.
  data?: string;
  difficulty: number;
  nonce: number;
}

export interface WorkerArgs {
  data: string;
  difficulty: number;
  nonce: number;
  threads: number;
}

// A spawner hands out workers that all run the same script. Which flavour of
// spawner we get depends on whether we could pre-fetch the worker source.
export interface WorkerSpawner {
  // spawn a single worker based on the template
  spawn: () => Worker;
  // demote to fallback/less efficient logic instead of using more efficient
  // logic.
  demote: () => boolean;
  // destroy this instance, manual cleanup logic so you don't leak Blob
  // instances, etc.
  dispose: () => void;
}

export const directSpawner = (webWorkerURL: string): WorkerSpawner => {
  return {
    spawn: () => new Worker(webWorkerURL),
    demote: () => false,
    dispose: () => {},
  };
};

/**
 * createWorkerSpawner fetches the worker source _once_ and hands back a
 * WorkerSpawner instance and hands back a spawner backed by a Blob URL
 * instead of making each worker fetch the worker code individually.
 *
 * Doing it this way prevents a single client with many threads fetching
 * the same worker source code n times for values that are fixed for the
 * entire lifetime of the deployment of Anubis at that version. The old
 * method made a client with 8 threads fetch the worker logic 8 times,
 * which made an overloaded server even more overloaded, causing spurious
 * failures, which cause challenges to fail 100% of the time. This is
 * bad for uptime.
 *
 * Annoyingly, `new Worker(url)` reports a load failure as an opaque event
 * that does not include the status code of the failed load, so there's
 * no real way to detect "503, back off and retry" from "404, you fetched
 * the wrong asset". To work around this we have to `fetch()` the contents
 * of the worker code and spawn it with a Blob.
 *
 * If anything goes wrong or the server's Content-Security-Policy (CSP)
 * forbids putting worker sources in Blobs, we fall back to the old behaviour
 * with the caveat that doing this is kinda hacky and terrible, but such is
 * life.
 */
export const createWorkerSpawner = async (
  webWorkerURL: string,
  signal: AbortSignal | null,
): Promise<WorkerSpawner> => {
  const direct: WorkerSpawner = directSpawner(webWorkerURL);

  let blobURL: string;
  try {
    const response = await fetchWithBackoff(webWorkerURL, { signal });
    blobURL = URL.createObjectURL(
      new Blob([await response.text()], { type: "text/javascript" }),
    );
  } catch (err) {
    if (isAbortError(err)) {
      throw err;
    }
    // Every retry failed. Fall through to the direct spawner anyway: the
    // browser may still have a cached copy that `fetch` did not surface, and a
    // long shot beats a guaranteed failure.
    console.warn(
      "anubis: could not pre-fetch worker source (server may be under attack) using direct spawner in the vain hope that this works",
      err,
    );
    return direct;
  }

  let useBlob = true;

  return {
    spawn: (): Worker => {
      if (useBlob) {
        try {
          return new Worker(blobURL);
        } catch (err) {
          // XXX(Xe): Chrome, Firefox, and WebKit won't trigger this, but it's best
          // to be defensive here.
          console.warn("anubis: blob worker rejected, using direct URL", err);
          useBlob = false;
        }
      }
      return new Worker(webWorkerURL);
    },
    demote: (): boolean => {
      if (!useBlob) {
        return false;
      }

      console.warn(
        "anubis: blob workers are not running, falling back to loading workers from their own URL (does this site's Content-Security-Policy allow blob: in worker-src?)",
      );
      useBlob = false;
      return true;
    },
    dispose: (): void => URL.revokeObjectURL(blobURL),
  };
};

/**
 * workerError pulls the error message out of a worker's failure report, or
 * returns null when the payload is an ordinary result.
 */
const workerError = (data: unknown): string | null => {
  if (data == null || typeof data !== "object") {
    return null;
  }
  const err = (data as { error?: unknown }).error;
  return err == null ? null : String(err);
};

/** defaultThreads is how many workers a challenge spawns when not told otherwise. */
export const defaultThreads = (): number =>
  Math.trunc(Math.max(getHardwareConcurrency() / 2, 1));

export interface RunWorkersOptions {
  // URL of the worker script to run.
  webWorkerURL: string;
  // Fields merged into every worker's postMessage payload. Each worker also
  // gets its own `nonce` (its lane) and the total `threads` count.
  message: Record<string, unknown>;
  threads: number;
  signal: AbortSignal | null;
  scale: number;
  progressCallback?: ProgressCallback;
}

/**
 * runWorkers fans a challenge out across `threads` workers and resolves with
 * the first solution any of them finds.
 *
 * Every challenge algorithm shares this because every one of them has the same
 * failure modes: a server too overloaded to serve the worker script, a
 * Content-Security-Policy that forbids blob: workers, and workers that die
 * partway through. Solving those once here keeps the algorithms from drifting
 * apart on the parts that decide whether a user gets through at all.
 */
export const runWorkers = async (
  opts: RunWorkersOptions,
): Promise<ProcessResult> => {
  const spawner = await createWorkerSpawner(opts.webWorkerURL, opts.signal);

  try {
    return await raceWorkers(spawner, opts);
  } finally {
    spawner.dispose();
  }
};

const raceWorkers = (
  spawner: WorkerSpawner,
  { message, threads, signal, scale, progressCallback }: RunWorkersOptions,
): Promise<ProcessResult> => {
  return new Promise((resolve, reject) => {
    let workers: Worker[] = [];
    let settled = false;
    let deadWorkers = 0;
    // XXX(Xe): sentinel for workers, if this is not set then a CSP refused
    // to load scripts normally.
    let anyOutput = false;

    const onAbort = () => {
      console.log("PoW aborted");
      cleanup();
      reject(new DOMException("Aborted", "AbortError"));
    };

    const stopWorkers = () => {
      workers.forEach((w) => {
        w.onmessage = null;
        w.onerror = null;
        w.terminate();
      });
      workers = [];
    };

    const cleanup = () => {
      if (settled) {
        return;
      }
      settled = true;
      stopWorkers();
      if (signal != null) {
        signal.removeEventListener("abort", onAbort);
      }
    };

    if (signal != null) {
      if (signal.aborted) {
        return onAbort();
      }
      signal.addEventListener("abort", onAbort, { once: true });
    }

    const startWorkers = () => {
      deadWorkers = 0;

      for (let i = 0; i < threads; i++) {
        let worker: Worker;
        try {
          worker = spawner.spawn();
        } catch (err) {
          // If we can't spawn workers, we're in a weird place. Throw an error
          // up the stack and kill the workers so they don't burn CPU and try
          // to send a message that won't be read.
          cleanup();
          reject(
            new Error(
              `anubis: could not start proof of work worker: ${err} (is your browser out of date?)`,
            ),
          );
          return;
        }

        workers.push(worker);

        worker.onmessage = (event) => {
          // Feed the watchdog.
          anyOutput = true;

          if (typeof event.data === "number") {
            progressCallback?.(event.data, scale);
            return;
          }

          // XXX(Xe): Workers register `async` message handlers, and a promise
          // that rejects inside one of those surfaces as an unhandledrejection
          // in the worker, not as an `error` event on this side. Workers report
          // those failures by hand so that they can't hang the page forever.
          const err = workerError(event.data);
          if (err !== null) {
            cleanup();
            reject(new Error(`anubis: proof of work worker failed: ${err}`));
            return;
          }

          cleanup();
          resolve(event.data);
        };

        const workerDied = (event: unknown) => {
          // XXX(Xe): Workers should generally be inerrant. If any of them has
          // died before producing any output then they have never ran because
          // the browser explicitly rejected the worker script due to an overly
          // strict Content-Security-Policy.
          //
          // Annoyingly, this isn't something that's easy to catch and validate
          // so we just check to make sure that we've seen at least _any_ output
          // from the worker and if we haven't then try to run the fallback
          // logic that makes N requests for N = getHardwareConcurrency().
          //
          // This can cause some issues with particularly overloaded servers
          // that can't route to Anubis due to biblical amounts of load, but
          // what can you do? At that point you kinda have to pick your battles
          // between reliability and consistency. Consistency is probably the
          // better tradeoff.
          if (!anyOutput && spawner.demote()) {
            console.warn(
              "anubis: proof of work workers died without running, respawning them",
            );
            stopWorkers();
            startWorkers();
            return;
          }

          deadWorkers++;
          console.warn(
            `anubis: proof of work worker died (${deadWorkers}/${threads})`,
            event,
          );

          // XXX(Xe): Make sure there's at least one worker left alive.
          //
          // One way to think about how the parallelism works here is that the
          // nonce space of challenge solutions is divided into one "lane" per
          // worker. In general solutions should be "dense" enough that losing
          // a few workers should be "fine enough".
          //
          // If all workers are dead, there is no way to search the entire nonce
          // space and thus the solution attempt has failed.
          if (deadWorkers < threads) {
            return;
          }

          cleanup();
          // XXX(Xe): If the worker fails to load, some browsers return non-error
          // Error values that we can't introspect properly. When we hit this kind
          // of horrible state, throw an actual error so that the challenge page
          // has a better error message than "undefined".
          reject(
            new Error(
              "anubis: all proof of work workers failed at runtime (file a bug?)",
            ),
          );
        };

        worker.onerror = workerDied;

        // A payload we can't deserialize is as fatal to this worker as a
        // crash, and it is silent otherwise.
        worker.onmessageerror = (event) => {
          console.warn(
            "anubis: proof of work worker sent a bad message",
            event,
          );
          workerDied(event);
        };

        worker.postMessage({
          ...message,
          nonce: i,
          threads,
        });
      }
    };

    startWorkers();
  });
};

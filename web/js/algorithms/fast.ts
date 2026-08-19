import {
  getHardwareConcurrency,
  ProgressCallback,
  ProcessOptions,
  ProcessResult,
  WorkerSpawner,
  createWorkerSpawner,
} from "@lib/worker";

// Choose worker based on secure context.
// Use the WebCrypto worker if the page is a secure context; otherwise fall back to pure‑JS.
const chooseWorkerMethod = (): "webcrypto" | "purejs" => {
  if (
    navigator.userAgent.includes("Firefox") ||
    navigator.userAgent.includes("Goanna")
  ) {
    console.log("Firefox detected, using pure-JS fallback");
    return "purejs";
  }

  return window.isSecureContext ? "webcrypto" : "purejs";
};

export default async function process(
  options: ProcessOptions,
  data: string,
  difficulty: number = 5,
  signal: AbortSignal | null = null,
  progressCallback?: ProgressCallback,
  threads: number = Math.trunc(Math.max(getHardwareConcurrency() / 2, 1)),
): Promise<ProcessResult> {
  console.debug("fast algo");

  const workerMethod = chooseWorkerMethod();
  const webWorkerURL = `${options.basePrefix}/.within.website/x/cmd/anubis/static/js/worker/sha256-${workerMethod}.mjs?cacheBuster=${options.version}`;

  const spawner = await createWorkerSpawner(webWorkerURL, signal);

  try {
    return await runWorkers(
      spawner,
      data,
      difficulty,
      signal,
      progressCallback,
      threads,
    );
  } finally {
    spawner.dispose();
  }
}

function runWorkers(
  spawner: WorkerSpawner,
  data: string,
  difficulty: number,
  signal: AbortSignal | null,
  progressCallback: ProgressCallback | undefined,
  threads: number,
): Promise<ProcessResult> {
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
            progressCallback?.(event.data);
          } else {
            cleanup();
            resolve(event.data);
          }
        };

        worker.onerror = (event) => {
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

        worker.postMessage({
          data,
          difficulty,
          nonce: i,
          threads,
        });
      }
    };

    startWorkers();
  });
}

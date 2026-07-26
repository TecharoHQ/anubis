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
    const workers: Worker[] = [];
    let settled = false;
    let deadWorkers = 0;

    const onAbort = () => {
      console.log("PoW aborted");
      cleanup();
      reject(new DOMException("Aborted", "AbortError"));
    };

    const cleanup = () => {
      if (settled) {
        return;
      }
      settled = true;
      workers.forEach((w) => w.terminate());
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

    for (let i = 0; i < threads; i++) {
      let worker: Worker;
      try {
        worker = spawner.spawn();
      } catch (err) {
        // If we can't spawn workers, we're in a weird place. Bail by cleaning
        // up and throwing an error up the stack so that the user can report
        // Terminate the workers we already started. Letting them leak leaves
        // spinning infinite loops behind that burn a core until navigation.
        cleanup();
        reject(
          new Error(`anubis: could not start proof of work worker: ${err} (is your browser out of date?)`),
        );
        return;
      }

      // Track the worker before doing anything else with it so cleanup() can
      // always reach it.
      workers.push(worker);

      worker.onmessage = (event) => {
        if (typeof event.data === "number") {
          progressCallback?.(event.data);
        } else {
          cleanup();
          resolve(event.data);
        }
      };

      worker.onerror = (event) => {
        deadWorkers++;
        console.warn(
          `anubis: proof of work worker died (${deadWorkers}/${threads})`,
          event,
        );

        // Losing a worker isn't fatal, it does mean that the nonce lane that
        // the worker covers isn't being covered anymore, but the other workers
        // will cover the other nonce lanes. This means that losing one lane
        // costs throughput but not correctness. Solutions generally should
        // be dense enough that survivors should still find one.
        //
        // However if all workers die, then the entire process is dead and we
        // need to surface this up to the user.
        if (deadWorkers < threads) {
          return;
        }

        cleanup();
        // If the worker fails to load, some browsers return non-error Error
        // values that we can't introspect properly. When we hit this kind of
        // horrible state, throw an actual error so that the challenge page
        // has a better error message than "undefined".
        reject(new Error("anubis: all proof of work workers failed at runtime (file a bug?)"));
      };

      worker.postMessage({
        data,
        difficulty,
        nonce: i,
        threads,
      });
    }
  });
}

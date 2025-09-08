type ProgressCallback = (nonce: number) => void;

interface ProcessOptions {
  basePrefix: string;
  version: string;
}

export default function process(
  options: ProcessOptions,
  data: string,
  difficulty: number = 5,
  signal: AbortSignal | null = null,
  progressCallback?: ProgressCallback,
  threads: number = Math.trunc(Math.max(navigator.hardwareConcurrency / 2, 1)),
): Promise<string> {
  console.debug("fast algo");

  let workerMethod = window.crypto !== undefined ? "webcrypto" : "purejs";

  if (navigator.userAgent.includes("Firefox") || navigator.userAgent.includes("Goanna")) {
    console.log("Firefox detected, using pure-JS fallback");
    workerMethod = "purejs";
  }

  return new Promise((resolve, reject) => {
    let webWorkerURL = `${options.basePrefix}/.within.website/x/cmd/anubis/static/js/worker/sha256-${workerMethod}.mjs?cacheBuster=${options.version}`;

    const workers: Worker[] = [];
    let settled = false;

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
      let worker = new Worker(webWorkerURL);

      worker.onmessage = (event) => {
        if (typeof event.data === "number") {
          progressCallback?.(event.data);
        } else {
          cleanup();
          resolve(event.data);
        }
      };

      worker.onerror = (event) => {
        cleanup();
        reject(event);
      };

      worker.postMessage({
        data,
        difficulty,
        nonce: i,
        threads,
      });

      workers.push(worker);
    }
  });
}
type ProgressCallback = (nonce: number) => void;

interface ProcessOptions {
  basePrefix: string;
  version: string;
}

const getHardwareConcurrency = () =>
  navigator.hardwareConcurrency !== undefined ? navigator.hardwareConcurrency : 1;

export default function process(
  options: ProcessOptions,
  data: string,
  difficulty: number = 5,
  signal: AbortSignal | null = null,
  progressCallback?: ProgressCallback,
  threads: number = Math.trunc(Math.max(getHardwareConcurrency() / 2, 1)),
): Promise<string> {
  console.debug("fast algo");

  // Choose worker based on secure context.
  // Use the WebCrypto worker if the page is a secure context; otherwise fall back to pure‑JS.
  let workerMethod: "webcrypto" | "purejs" = "purejs";
  if (window.isSecureContext) {
    workerMethod = "webcrypto";
  }

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

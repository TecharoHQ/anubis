export default function process(
  { basePrefix, version },
  data,
  difficulty = 5,
  signal = null,
  progressCallback = null,
  threads = Math.max(navigator.hardwareConcurrency / 2, 1),
) {
  console.debug("fast algo");

  let workerMethod = window.crypto !== undefined ? "webcrypto" : "purejs";

  if (navigator.userAgent.includes("Firefox") || navigator.userAgent.includes("Goanna")) {
    console.log("Firefox detected, using pure-JS fallback");
    workerMethod = "purejs";
  }

  return new Promise((resolve, reject) => {
    // Build the worker URL properly using URL constructor to avoid parsing issues
    const workerPath = `/.within.website/x/cmd/anubis/static/js/worker/sha256-${workerMethod}.mjs`;
    const searchParams = new URLSearchParams({ cacheBuster: version });
    
    // Use URL constructor to ensure proper URL formation
    const webWorkerURL = new URL(workerPath + '?' + searchParams.toString(), window.location.origin).toString();

    console.log('Worker URL:', webWorkerURL);

    const workers = [];
    let settled = false;

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

    const onAbort = () => {
      console.log("PoW aborted");
      cleanup();
      reject(new DOMException("Aborted", "AbortError"));
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
const determineThreadCount = () => {
  if (navigator.userAgent.includes("Firefox")) {
    return Math.min(navigator.hardwareConcurrency, 4);
  }

  if (!!navigator.hardwareConcurrency) {
    return navigator.hardwareConcurrency;
  }

  return 1;
};

export default function process(
  { basePrefix, version },
  data,
  difficulty = 5,
  signal = null,
  progressCallback = null,
  threads = determineThreadCount(),
) {
  return new Promise((resolve, reject) => {
    let webWorkerURL = `${basePrefix}/.within.website/x/cmd/anubis/static/js/worker/fast.mjs?cacheBuster=${version}`;

    const workers = [];
    const terminate = () => {
      workers.forEach((w) => w.terminate());
      if (signal != null) {
        // clean up listener to avoid memory leak
        signal.removeEventListener("abort", terminate);
        if (signal.aborted) {
          console.log("PoW aborted");
          reject(false);
        }
      }
    };
    if (signal != null) {
      signal.addEventListener("abort", terminate, { once: true });
    }

    for (let i = 0; i < threads; i++) {
      let worker = new Worker(webWorkerURL);

      worker.onmessage = (event) => {
        if (typeof event.data === "number") {
          progressCallback?.(event.data);
        } else {
          terminate();
          resolve(event.data);
        }
      };

      worker.onerror = (event) => {
        terminate();
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

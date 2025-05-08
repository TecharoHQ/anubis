// https://dev.to/ratmd/simple-proof-of-work-in-javascript-3kgm

export default function process(
  { basePrefix, version },
  data,
  difficulty = 5,
  signal = null,
  progressCallback = null,
  _threads = 1,
) {
  return new Promise((resolve, reject) => {
    let worker = new Worker(`${basePrefix}/.within.website/x/cmd/anubis/static/js/worker/slow.mjs?cacheBuster=${version}`);
    const terminate = () => {
      worker.terminate();
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
      difficulty
    });
  });
}


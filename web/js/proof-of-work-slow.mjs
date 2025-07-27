// https://dev.to/ratmd/simple-proof-of-work-in-javascript-3kgm

export default function process(
  data,
  difficulty = 5,
  signal = null,
  progressCallback = null,
  _threads = 1,
) {
  console.debug("slow algo");
  return new Promise((resolve, reject) => {
    let webWorkerURL = URL.createObjectURL(
      new Blob(["(", processTask(), ")()"], { type: "application/javascript" }),
    );

    let worker = new Worker(webWorkerURL);
    let settled = false;

    const cleanup = () => {
      if (settled) return;
      settled = true;
      worker.terminate();
      if (signal != null) {
        signal.removeEventListener("abort", onAbort);
      }
      URL.revokeObjectURL(webWorkerURL);
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
    });
  });
}

function processTask() {
  return function () {
    const sha256 = (text) => {
      const encoded = new TextEncoder().encode(text);
      return crypto.subtle.digest("SHA-256", encoded.buffer).then((result) =>
        Array.from(new Uint8Array(result))
          .map((c) => c.toString(16).padStart(2, "0"))
          .join(""),
      );
    };

    addEventListener("message", async (event) => {
      let data = event.data.data;
      let difficulty = event.data.difficulty;

      let hash;
      let nonce = 0;
      do {
        if ((nonce & 1023) === 0) {
          postMessage(nonce);
        }
        hash = await sha256(data + nonce++);
      } while (
        hash.substring(0, difficulty) !== Array(difficulty + 1).join("0")
      );

      nonce -= 1; // last nonce was post-incremented

      postMessage({
        hash,
        data,
        difficulty,
        nonce,
      });
    });
  }.toString();
}

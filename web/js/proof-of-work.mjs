export default function process(
  data,
  difficulty = 5,
  signal = null,
  progressCallback = null,
  threads = Math.max(navigator.hardwareConcurrency / 2, 1),
) {
  console.debug("fast algo");
  return new Promise((resolve, reject) => {
    let webWorkerURL = URL.createObjectURL(
      new Blob(["(", processTask(), ")()"], { type: "application/javascript" }),
    );

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

function processTask() {
  return function () {
    const sha256 = (text) => {
      const encoded = new TextEncoder().encode(text);
      return crypto.subtle.digest("SHA-256", encoded.buffer);
    };

    function uint8ArrayToHexString(arr) {
      return Array.from(arr)
        .map((c) => c.toString(16).padStart(2, "0"))
        .join("");
    }

    addEventListener("message", async (event) => {
      let data = event.data.data;
      let difficulty = event.data.difficulty;
      let hash;
      let nonce = event.data.nonce;
      let threads = event.data.threads;

      const threadId = nonce;
      let localIterationCount = 0;

      while (true) {
        const currentHash = await sha256(data + nonce);
        const thisHash = new Uint8Array(currentHash);
        let valid = true;

        for (let j = 0; j < difficulty; j++) {
          const byteIndex = Math.floor(j / 2); // which byte we are looking at
          const nibbleIndex = j % 2; // which nibble in the byte we are looking at (0 is high, 1 is low)

          let nibble =
            (thisHash[byteIndex] >> (nibbleIndex === 0 ? 4 : 0)) & 0x0f; // Get the nibble

          if (nibble !== 0) {
            valid = false;
            break;
          }
        }

        if (valid) {
          hash = uint8ArrayToHexString(thisHash);
          console.log(hash);
          break;
        }

        nonce += threads;

        // send a progress update every 1024 iterations so that the user can be informed of
        // the state of the challenge.
        if (threadId == 0 && localIterationCount === 1024) {
          postMessage(nonce);
          localIterationCount = 0;
        }
        localIterationCount++;
      }

      postMessage({
        hash,
        data,
        difficulty,
        nonce,
      });
    });
  }.toString();
}

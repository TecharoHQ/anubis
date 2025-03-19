// https://dev.to/ratmd/simple-proof-of-work-in-javascript-3kgm

export function process(data, difficulty = 5, threads = navigator.hardwareConcurrency) {
  return new Promise((resolve, reject) => {
    let webWorkerURL = URL.createObjectURL(new Blob([
      '(', processTask(), ')()'
    ], { type: 'application/javascript' }));

    const workers = [];

    for (let i = 0; i < threads; i++) {
      let worker = new Worker(webWorkerURL);

      worker.onmessage = (event) => {
        workers.forEach(worker => worker.terminate());
        worker.terminate();
        resolve(event.data);
      };

      worker.onerror = (event) => {
        worker.terminate();
        reject();
      };

      worker.postMessage({
        data,
        difficulty,
        nonce: 1000000 * i,
      });

      workers.push(worker);
    }

    URL.revokeObjectURL(webWorkerURL);
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

    addEventListener('message', async (event) => {
      let data = event.data.data;
      let difficulty = event.data.difficulty;
      let hash;
      let nonce = event.data.nonce || 0;

      while (true) {
        const currentHash = await sha256(data + nonce++);
        const thisHash = new Uint8Array(currentHash);
        let valid = true;

        for (let j = 0; j < difficulty; j++) {
          const byteIndex = Math.floor(j / 2); // which byte we are looking at
          const nibbleIndex = j % 2; // which nibble in the byte we are looking at (0 is high, 1 is low)

          let nibble = (thisHash[byteIndex] >> (nibbleIndex === 0 ? 4 : 0)) & 0x0F; // Get the nibble

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
      }

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


import { u } from "../xeact.mjs";

export default function process(
  data,
  difficulty = 16,
  signal = null,
  pc = null,
  threads = (navigator.hardwareConcurrency || 1),
) {
  return new Promise(async (resolve, reject) => {
    let webWorkerURL = URL.createObjectURL(new Blob([
      '(', processTask(), ')()'
    ], { type: 'application/javascript' }));

    const module = await fetch(u("/.within.website/x/cmd/anubis/static/wasm/sha256.wasm"))
      .then(resp => WebAssembly.compileStreaming(resp));

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
          pc?.(event.data);
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
        module,
      });

      workers.push(worker);
    }

    URL.revokeObjectURL(webWorkerURL);
  });
}

function processTask() {
  return function () {
    addEventListener('message', async (event) => {
      const importObject = {
        anubis: {
          anubis_update_nonce: (nonce) => postMessage(nonce),
        }
      };

      const instance = await WebAssembly.instantiate(event.data.module, importObject);

      // Get exports
      const {
        anubis_work,
        data_ptr,
        result_hash_ptr,
        result_hash_size,
        set_data_length,
        memory
      } = instance.exports;

      function uint8ArrayToHex(arr) {
        return Array.from(arr)
          .map((c) => c.toString(16).padStart(2, "0"))
          .join("");
      }

      function hexToUint8Array(hexString) {
        // Remove whitespace and optional '0x' prefix
        hexString = hexString.replace(/\s+/g, '').replace(/^0x/, '');

        // Check for valid length
        if (hexString.length % 2 !== 0) {
          throw new Error('Invalid hex string length');
        }

        // Check for valid characters
        if (!/^[0-9a-fA-F]+$/.test(hexString)) {
          throw new Error('Invalid hex characters');
        }

        // Convert to Uint8Array
        const byteArray = new Uint8Array(hexString.length / 2);
        for (let i = 0; i < byteArray.length; i++) {
          const byteValue = parseInt(hexString.substr(i * 2, 2), 16);
          byteArray[i] = byteValue;
        }

        return byteArray;
      }

      // Write data to buffer
      function writeToBuffer(data) {
        if (data.length > 1024) throw new Error("Data exceeds buffer size");

        // Get pointer and create view
        const offset = data_ptr();
        const buffer = new Uint8Array(memory.buffer, offset, data.length);

        // Copy data
        buffer.set(data);

        // Set data length
        set_data_length(data.length);
      }

      function readFromChallenge() {
        const offset = result_hash_ptr();
        const buffer = new Uint8Array(memory.buffer, offset, result_hash_size());

        return buffer;
      }

      let data = event.data.data;
      let difficulty = event.data.difficulty;
      let hash;
      let nonce = event.data.nonce;
      let interand = event.data.threads;

      writeToBuffer(hexToUint8Array(data));

      nonce = anubis_work(difficulty, nonce, interand);
      const challenge = readFromChallenge();

      data = uint8ArrayToHex(challenge);

      postMessage({
        hash: data,
        difficulty,
        nonce,
      });
    });
  }.toString();
}


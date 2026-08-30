import {
  Args,
  AnubisExports,
  uint8ArrayToHex,
  hexToUint8Array,
} from "@lib/wasm";

addEventListener("message", async (event: MessageEvent<Args>) => {
  try {
    const { data, difficulty, threads, module } = event.data;
    let { nonce } = event.data;

    const importObject = {
      anubis: {
        anubis_update_nonce: (nonce: number) => postMessage(nonce),
      },
    };

    if (nonce !== 0) {
      importObject.anubis.anubis_update_nonce = (_) => {};
    }

    // instantiate() resolves to an Instance (not a ResultObject) when passed a compiled Module.
    const instance = await WebAssembly.instantiate(module, importObject);

    const {
      anubis_work,
      data_ptr,
      result_hash_ptr,
      result_hash_size,
      set_data_length,
      memory,
    } = instance.exports as unknown as AnubisExports;

    function writeToBuffer(data: Uint8Array) {
      if (data.length > 4096) {
        throw new Error("Data exceeds buffer size");
      }

      const offset = data_ptr();
      const buffer = new Uint8Array(memory.buffer, offset, data.length);

      buffer.set(data);
      set_data_length(data.length);
    }

    function readFromChallenge() {
      const offset = result_hash_ptr();
      const buffer = new Uint8Array(memory.buffer, offset, result_hash_size());

      return buffer;
    }

    writeToBuffer(hexToUint8Array(data));

    nonce = anubis_work(difficulty, nonce, threads);
    const challenge = readFromChallenge();
    const result = uint8ArrayToHex(challenge);

    postMessage({
      hash: result,
      difficulty,
      nonce,
    });
  } catch (err) {
    // A rejection inside an async message handler shows up as an
    // unhandledrejection in this worker, never as an error event on the page.
    // Report it by hand so the challenge fails loudly instead of hanging.
    postMessage({ error: String(err) });
  }
});

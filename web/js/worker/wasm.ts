export interface Args {
  data: string;
  difficulty: number;
  nonce: number;
  threads: number;
  module: BufferSource;
}

interface AnubisExports {
  anubis_work: (difficulty: number, initialNonce: number, threads: number) => number;
  data_ptr: () => number;
  result_hash_ptr: () => number;
  result_hash_size: () => number;
  set_data_length: (len: number) => void;
  memory: WebAssembly.Memory;
}

addEventListener("message", async (event: MessageEvent<Args>) => {
  const { data, difficulty, threads, module } = event.data;
  let { nonce } = event.data;

  const importObject = {
    anubis: {
      anubis_update_nonce: (nonce: number) => postMessage(nonce),
    }
  };

  if (nonce !== 0) {
    importObject.anubis.anubis_update_nonce = (_) => { };
  }

  const obj = await WebAssembly.instantiate(module, importObject);

  const {
    anubis_work,
    data_ptr,
    result_hash_ptr,
    result_hash_size,
    set_data_length,
    memory
  } = (obj as unknown as any).exports as unknown as AnubisExports;
  function uint8ArrayToHex(arr: Uint8Array) {
    return Array.from(arr)
      .map((c) => c.toString(16).padStart(2, "0"))
      .join("");
  }

  function hexToUint8Array(hexString: string): Uint8Array {
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
  function writeToBuffer(data: Uint8Array) {
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

  writeToBuffer(hexToUint8Array(data));

  nonce = anubis_work(difficulty, nonce, threads);
  const challenge = readFromChallenge();
  const result = uint8ArrayToHex(challenge);

  postMessage({
    hash: result,
    difficulty,
    nonce,
  });
});
import * as argon2id from "../gen/wasm2js/argon2id.wasm.js";
import * as hashx from "../gen/wasm2js/hashx.wasm.js";
import * as sha256 from "../gen/wasm2js/sha256.wasm.js";

import { Wasm2JSArgs, AnubisExports, uint8ArrayToHex, hexToUint8Array } from "@lib/wasm";

const algorithms: Record<string, AnubisExports> = {
  "argon2id": argon2id,
  "hashx": hashx,
  "sha256": sha256,
};

addEventListener("message", async (event: MessageEvent<Wasm2JSArgs>) => {
  const { data, difficulty, threads, algorithm } = event.data;
  let { nonce } = event.data;

  const obj = algorithms[algorithm];
  if (obj == undefined) {
    throw new Error(`unknown algorithm ${algorithm}, file a bug please`);
  }

  const {
    anubis_work,
    data_ptr,
    result_hash_ptr,
    result_hash_size,
    set_data_length,
    memory
  } = obj;

  // Write data to buffer
  function writeToBuffer(data: Uint8Array) {
    if (data.length > 1024) {
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
});
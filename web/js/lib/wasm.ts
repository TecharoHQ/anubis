import { WorkerArgs } from "./worker";

export interface AnubisExports {
  anubis_work: (
    difficulty: number,
    initialNonce: number,
    threads: number,
  ) => number;
  data_ptr: () => number;
  result_hash_ptr: () => number;
  result_hash_size: () => number;
  set_data_length: (len: number) => void;
  memory: WebAssembly.Memory;
}

export interface Args extends WorkerArgs {
  module: WebAssembly.Module;
}

export interface Wasm2JSArgs extends WorkerArgs {
  algorithm: string;
}

export function uint8ArrayToHex(arr: Uint8Array) {
  return Array.from(arr)
    .map((c) => c.toString(16).padStart(2, "0"))
    .join("");
}

export function hexToUint8Array(hexString: string): Uint8Array {
  // Remove whitespace and optional '0x' prefix
  hexString = hexString.replace(/\s+/g, "").replace(/^0x/, "");

  // Check for valid length
  if (hexString.length % 2 !== 0) {
    throw new Error("Invalid hex string length");
  }

  // Check for valid characters
  if (!/^[0-9a-fA-F]+$/.test(hexString)) {
    throw new Error("Invalid hex characters");
  }

  // Convert to Uint8Array
  const byteArray = new Uint8Array(hexString.length / 2);
  for (let i = 0; i < byteArray.length; i++) {
    const byteValue = parseInt(hexString.substr(i * 2, 2), 16);
    byteArray[i] = byteValue;
  }

  return byteArray;
}

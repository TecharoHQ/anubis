// The generated types for .wasm.js files match the C ABI for
// the Rust modules. This makes Typescript shut up and accept it.

declare module "*.wasm.js" {
  type AnubisExports = import("@lib/wasm").AnubisExports;

  export const anubis_work: AnubisExports["anubis_work"];
  export const data_ptr: AnubisExports["data_ptr"];
  export const result_hash_ptr: AnubisExports["result_hash_ptr"];
  export const result_hash_size: AnubisExports["result_hash_size"];
  export const set_data_length: AnubisExports["set_data_length"];
  export const memory: AnubisExports["memory"];
}

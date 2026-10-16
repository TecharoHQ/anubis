import { readFile } from "node:fs/promises";
import { WASI } from "node:wasi";
import { argv, cwd, env, exit } from "node:process";

const [wasmBinName, ...wasmArgs] = argv.slice(2);

if (!wasmBinName) {
  console.error("usage: node execer.mjs <bin.wasm> [args...]");
  exit(2);
}

const wasi = new WASI({
  version: "preview1",
  args: [wasmBinName, ...wasmArgs],
  env,
  preopens: {
    "/": cwd(),
  },
  returnOnExit: true,
});

const wasm = await WebAssembly.compile(
  await readFile(wasmBinName),
);
const instance = await WebAssembly.instantiate(wasm, wasi.getImportObject());

exit(wasi.start(instance));
import { ProgressCallback, ProcessOptions, getHardwareConcurrency } from "@lib/worker";
import { u } from "@lib/xeact.mjs";
import { simd } from "wasm-feature-detect";
import isWASMSupported, { WASMUnsupportedError } from "@lib/wasm-supported";
import wasm2js from "./wasm2js";

export default function process(
  options: ProcessOptions,
  data: string,
  difficulty: number = 5,
  signal: AbortSignal | null = null,
  progressCallback?: ProgressCallback,
  threads: number = Math.trunc(Math.max(getHardwareConcurrency() / 2, 1)),
): Promise<string> {
  const { basePrefix, version, algorithm } = options;

  if (!isWASMSupported) {
    return wasm2js(options, data, difficulty, signal, progressCallback, threads);
  }

  return new Promise(async (resolve, reject) => {
    let wasmFeatures = "baseline";

    if (await simd()) {
      wasmFeatures = "simd128";
    }

    let module = await fetch(u(`${basePrefix}/.within.website/x/cmd/anubis/static/wasm/${wasmFeatures}/${algorithm}.wasm?cacheBuster=${version}`))
      .then(x => WebAssembly.compileStreaming(x));

    const webWorkerURL = `${basePrefix}/.within.website/x/cmd/anubis/static/js/worker/wasm.mjs?cacheBuster=${version}`;

    const workers: Worker[] = [];
    let settled = false;

    const onAbort = () => {
      console.log("PoW aborted");
      cleanup();
      reject(new DOMException("Aborted", "AbortError"));
    };

    const cleanup = () => {
      if (settled) {
        return;
      }
      settled = true;
      workers.forEach((w) => w.terminate());
      if (signal != null) {
        signal.removeEventListener("abort", onAbort);
      }
    };

    if (signal != null) {
      if (signal.aborted) {
        return onAbort();
      }
      signal.addEventListener("abort", onAbort, { once: true });
    }

    for (let i = 0; i < threads; i++) {
      let worker = new Worker(webWorkerURL);
      workers.push(worker);

      worker.onmessage = (event) => {
        if (typeof event.data === "number") {
          progressCallback?.(event.data);
        } else {
          cleanup();
          resolve(event.data);
        }
      }

      worker.onerror = (event) => {
        cleanup();
        reject(event);
      }

      worker.postMessage({
        data,
        difficulty,
        nonce: i,
        threads,
        algorithm,
        module,
      });
    }
  });
};

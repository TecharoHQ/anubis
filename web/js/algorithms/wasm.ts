import { u } from "../../lib/xeact";
import { simd } from "wasm-feature-detect";
// import { compile } from '@haribala/wasm2js';

type ProgressCallback = (nonce: number | string) => void;

interface ProcessOptions {
  basePrefix: string;
  version: string;
  algorithm: string;
}

const getHardwareConcurrency = () =>
  navigator.hardwareConcurrency !== undefined ? navigator.hardwareConcurrency : 1;

// https://stackoverflow.com/questions/47879864/how-can-i-check-if-a-browser-supports-webassembly
const isWASMSupported = (() => {
  try {
    if (typeof WebAssembly === "object"
      && typeof WebAssembly.instantiate === "function") {
      const module = new WebAssembly.Module(Uint8Array.of(0x0, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00));
      if (module instanceof WebAssembly.Module)
        return new WebAssembly.Instance(module) instanceof WebAssembly.Instance;
    }
  } catch (e) {
    return false;
  }
  return false;
})();

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
    throw new Error("WebAssembly is not supported on this platform. Please report ALL details about your browser, environment, OS, CPU, device vendor, and other details to https://github.com/TecharoHQ/anubis/issues/1116. Remember: being polite means you get this fixed.");
  }

  return new Promise(async (resolve, reject) => {
    let wasmFeatures = "baseline";

    if (await simd()) {
      wasmFeatures = "simd128";
    }

    const module = await fetch(u(`${basePrefix}/.within.website/x/cmd/anubis/static/wasm/${wasmFeatures}/${algorithm}.wasm?cacheBuster=${version}`))
      .then(x => WebAssembly.compileStreaming(x));

    const webWorkerURL = `${options.basePrefix}/.within.website/x/cmd/anubis/static/js/worker/wasm.mjs?cacheBuster=${version}`;

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
        module,
      });
    }
  });
};


import { u } from "../../lib/xeact";
import { simd } from "wasm-feature-detect";

type ProgressCallback = (nonce: number) => void;

interface ProcessOptions {
  basePrefix: string;
  version: string;
  algorithm: string;
}

const getHardwareConcurrency = () =>
  navigator.hardwareConcurrency !== undefined ? navigator.hardwareConcurrency : 1;

export default function process(
  options: ProcessOptions,
  data: string,
  difficulty: number = 5,
  signal: AbortSignal | null = null,
  progressCallback?: ProgressCallback,
  threads: number = Math.trunc(Math.max(getHardwareConcurrency() / 2, 1)),
): Promise<string> {
  console.debug(options);
  const { basePrefix, version, algorithm } = options;

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


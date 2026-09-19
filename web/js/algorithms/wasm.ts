import {
  ProgressCallback,
  ProcessOptions,
  ProcessResult,
  defaultThreads,
  runWorkers,
} from "@lib/worker";
import { fetchWithBackoff } from "@lib/backoff";
import { simd } from "wasm-feature-detect";
import isWASMSupported from "@lib/wasm-supported";
import wasm2js from "./wasm2js";

// compileModule fetches and compiles the WebAssembly module for an algorithm.
//
// The module comes from the same server that is already being hammered, so it
// gets the same exponential backoff treatment as every other challenge asset.
const compileModule = async (
  url: string,
  signal: AbortSignal | null,
): Promise<WebAssembly.Module> => {
  const response = await fetchWithBackoff(url, { signal });

  try {
    return await WebAssembly.compileStreaming(response.clone());
  } catch (err) {
    // XXX(Xe): compileStreaming insists on an `application/wasm` Content-Type.
    // Anubis serves that, but middleware in front of Anubis has been known to
    // rewrite it. Buffering the whole module works regardless.
    console.warn(
      "anubis: streaming WebAssembly compilation failed, buffering instead",
      err,
    );
    return await WebAssembly.compile(await response.arrayBuffer());
  }
};

export default async function process(
  options: ProcessOptions,
  data: string,
  difficulty: number = 5,
  signal: AbortSignal | null = null,
  progressCallback?: ProgressCallback,
  scale: number = 2,
  threads: number = defaultThreads(),
): Promise<ProcessResult> {
  const { basePrefix, version, algorithm } = options;

  if (!isWASMSupported) {
    return wasm2js(
      options,
      data,
      difficulty,
      signal,
      progressCallback,
      threads,
    );
  }

  const wasmFeatures = (await simd()) ? "simd128" : "baseline";
  const module = await compileModule(
    `${basePrefix}/.within.website/x/cmd/anubis/static/wasm/${wasmFeatures}/${algorithm}.wasm?cacheBuster=${version}`,
    signal,
  );

  return runWorkers({
    webWorkerURL: `${basePrefix}/.within.website/x/cmd/anubis/static/js/worker/wasm.mjs?cacheBuster=${version}`,
    message: { data, difficulty, algorithm, module },
    threads,
    signal,
    progressCallback,
    scale,
  });
}

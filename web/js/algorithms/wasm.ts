import {
  ProgressCallback,
  ProcessOptions,
  ProcessResult,
  defaultThreads,
  runWorkers,
} from "@lib/worker";
import { fetchWithBackoff } from "@lib/backoff";
import {
  bulkMemory,
  signExtensions,
  mutableGlobals,
  saturatedFloatToInt,
  simd,
  multiValue,
} from "wasm-feature-detect";
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

const supportedTiers = async (): Promise<string[]> => {
  // Probe failures are unsupported features, not challenge failures.
  const supported = async (probe: () => Promise<boolean>): Promise<boolean> => {
    try {
      return await probe();
    } catch {
      return false;
    }
  };
  const baseline = await Promise.all(
    [bulkMemory, signExtensions, mutableGlobals, saturatedFloatToInt].map(
      supported,
    ),
  );
  if (!baseline.every(Boolean)) {
    return ["mvp"];
  }

  const vector = await Promise.all([simd, multiValue].map(supported));
  return vector.every(Boolean)
    ? ["simd128", "baseline", "mvp"]
    : ["baseline", "mvp"];
};

const compileAlgorithm = async (
  { basePrefix, version, algorithm }: ProcessOptions,
  signal: AbortSignal | null,
): Promise<WebAssembly.Module> => {
  const tiers = await supportedTiers();
  for (let index = 0; index < tiers.length; index++) {
    const tier = tiers[index];
    try {
      return await compileModule(
        `${basePrefix}/.within.website/x/cmd/anubis/static/wasm/${tier}/${algorithm}.wasm?cacheBuster=${version}`,
        signal,
      );
    } catch (err) {
      if (
        !(err instanceof WebAssembly.CompileError) ||
        signal?.aborted ||
        index === tiers.length - 1
      ) {
        throw err;
      }
      console.warn(
        `anubis: ${tier} WebAssembly compilation failed, trying ${tiers[index + 1]}`,
        err,
      );
    }
  }
  throw new Error("anubis: no eligible WebAssembly tier");
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

  const module = await compileAlgorithm(options, signal);

  return runWorkers({
    webWorkerURL: `${basePrefix}/.within.website/x/cmd/anubis/static/js/worker/wasm.mjs?cacheBuster=${version}`,
    message: { data, difficulty, algorithm, module },
    threads,
    signal,
    progressCallback,
    scale,
  });
}

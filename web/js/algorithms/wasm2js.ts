import {
  ProgressCallback,
  ProcessOptions,
  ProcessResult,
  defaultThreads,
  runWorkers,
} from "@lib/worker";

export default function process(
  options: ProcessOptions,
  data: string,
  difficulty: number = 5,
  signal: AbortSignal | null = null,
  progressCallback?: ProgressCallback,
  threads: number = defaultThreads(),
): Promise<ProcessResult> {
  const { basePrefix, version, algorithm } = options;

  return runWorkers({
    webWorkerURL: `${basePrefix}/.within.website/x/cmd/anubis/static/js/worker/wasm2js.mjs?cacheBuster=${version}`,
    message: { data, difficulty, algorithm },
    threads,
    signal,
    progressCallback,
  });
}

import {
  ProgressCallback,
  ProcessOptions,
  ProcessResult,
  defaultThreads,
  runWorkers,
} from "@lib/worker";

export default async function process(
  options: ProcessOptions,
  data: string,
  difficulty: number = 5,
  signal: AbortSignal | null = null,
  progressCallback?: ProgressCallback,
  threads: number = defaultThreads(),
): Promise<ProcessResult> {
  console.debug("fast algo");

  return runWorkers({
    webWorkerURL: `${options.basePrefix}/.within.website/x/cmd/anubis/static/js/worker/sha256.mjs?cacheBuster=${options.version}`,
    message: { data, difficulty },
    threads,
    signal,
    progressCallback,
  });
}

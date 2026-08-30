import {
  ProgressCallback,
  ProcessOptions,
  ProcessResult,
  defaultThreads,
  runWorkers,
} from "@lib/worker";

// Choose worker based on secure context.
// Use the WebCrypto worker if the page is a secure context; otherwise fall back to pure‑JS.
const chooseWorkerMethod = (): "webcrypto" | "purejs" => {
  if (
    navigator.userAgent.includes("Firefox") ||
    navigator.userAgent.includes("Goanna")
  ) {
    console.log("Firefox detected, using pure-JS fallback");
    return "purejs";
  }

  return window.isSecureContext ? "webcrypto" : "purejs";
};

export default async function process(
  options: ProcessOptions,
  data: string,
  difficulty: number = 5,
  signal: AbortSignal | null = null,
  progressCallback?: ProgressCallback,
  threads: number = defaultThreads(),
): Promise<ProcessResult> {
  console.debug("fast algo");

  const workerMethod = chooseWorkerMethod();

  return runWorkers({
    webWorkerURL: `${options.basePrefix}/.within.website/x/cmd/anubis/static/js/worker/sha256-${workerMethod}.mjs?cacheBuster=${options.version}`,
    message: { data, difficulty },
    threads,
    signal,
    progressCallback,
  });
}

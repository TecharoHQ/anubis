import { fetchWithBackoff, isAbortError } from "@lib/backoff";

export const getHardwareConcurrency = () =>
  navigator.hardwareConcurrency !== undefined
    ? navigator.hardwareConcurrency
    : 1;

export type ProgressCallback = (nonce: number) => void;

export interface ProcessOptions {
  basePrefix: string;
  version: string;
}

export interface ProcessResult {
  hash: string;
  data: string;
  difficulty: number;
  nonce: number;
}

export interface WorkerArgs {
  data: string;
  difficulty: number;
  nonce: number;
  threads: number;
};

// A spawner hands out workers that all run the same script. Which flavour of
// spawner we get depends on whether we could pre-fetch the worker source.
export interface WorkerSpawner {
  // spawn a single worker based on the template
  spawn: () => Worker;
  // destroy this instance, manual cleanup logic so you don't leak Blob
  // instances, etc.
  dispose: () => void;
}

export const directSpawner = (webWorkerURL: string): WorkerSpawner => {
  return {
    spawn: () => new Worker(webWorkerURL),
    dispose: () => { },
  };
}

/**
 * createWorkerSpawner fetches the worker source _once_ and hands back a
 * WorkerSpawner instance and hands back a spawner backed by a Blob URL
 * instead of making each worker fetch the worker code individually.
 *
 * Doing it this way prevents a single client with many threads fetching
 * the same worker source code n times for values that are fixed for the
 * entire lifetime of the deployment of Anubis at that version. The old
 * method made a client with 8 threads fetch the worker logic 8 times,
 * which made an overloaded server even more overloaded, causing spurious
 * failures, which cause challenges to fail 100% of the time. This is
 * bad for uptime.
 * 
 * Annoyingly, `new Worker(url)` reports a load failure as an opaque event
 * that does not include the status code of the failed load, so there's
 * no real way to detect "503, back off and retry" from "404, you fetched
 * the wrong asset". To work around this we have to `fetch()` the contents
 * of the worker code and spawn it with a Blob.
 * 
 * If anything goes wrong or the server's Content-Security-Policy forbids
 * putting worker sources in Blobs, we fall back to the old behaviour with
 * the caveat that doing this is kinda hacky and terrible, but such is life.
 */
export const createWorkerSpawner = async (
  webWorkerURL: string,
  signal: AbortSignal | null,
): Promise<WorkerSpawner> => {
  const direct: WorkerSpawner = directSpawner(webWorkerURL);

  let blobURL: string;
  try {
    const response = await fetchWithBackoff(webWorkerURL, { signal });
    blobURL = URL.createObjectURL(
      new Blob([await response.text()], { type: "text/javascript" }),
    );
  } catch (err) {
    if (isAbortError(err)) {
      throw err;
    }
    // Every retry failed. Fall through to the direct spawner anyway: the
    // browser may still have a cached copy that `fetch` did not surface, and a
    // long shot beats a guaranteed failure.
    console.warn("anubis: could not pre-fetch worker source (server may be under attack) using direct spawner in the vain hope that this works", err);
    return direct;
  }

  let useBlob = true;

  return {
    spawn: () => {
      if (useBlob) {
        try {
          return new Worker(blobURL);
        } catch (err) {
          // Most likely a CSP that forbids blob: workers.
          console.warn("anubis: blob worker rejected, using direct URL", err);
          useBlob = false;
        }
      }
      return new Worker(webWorkerURL);
    },
    dispose: () => URL.revokeObjectURL(blobURL),
  };
};
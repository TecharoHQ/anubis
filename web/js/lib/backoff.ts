// Retry helpers for fetching challenge assets.
//
// In order to load things reliably, Anubis needs to have the ability to load
// JavaScript and other assets using exponential backoff as the assets every
// challenge page needs come from the same server that's also being hammered.
// Due to facts and circumstances dating back to the legacy design of the proof
// of work client code, any failure of loading any assets causes the entire
// process to fail, which causes an error for the client, which makes the user
// refresh the page, which adds yet more load.
//
// These helpers make those fetches survive transient failures at the cost of
// using sleep-based exponential backoff, which is an acceptable tradeoff.

export interface BackoffOptions {
  attempts?: number;
  baseDelayMs?: number;
  maxDelayMs?: number;
  signal?: AbortSignal | null;
}

const DEFAULT_ATTEMPTS = 5;
const DEFAULT_BASE_DELAY_MS = 750;
const DEFAULT_MAX_DELAY_MS = 10000;

// HTTP status codes that make the request work retrying. Everything else is
// considered a permanent error and not retried.
const RETRYABLE_STATUSES = [
  408, // Request Timeout
  425, // Too Early
  429, // Too Many Requests
  500, // Internal Server Error
  502, // Bad Gateway
  503, // Service Unavailable
  504, // Gateway Timeout
];

// XXX(Xe): we need to check the exception by name rather than by
// `instanceof DOMException` because not every engine that can abort a `fetch`
// throws a `DOMException` to signal it. I hate it too.
export const isAbortError = (err: unknown): boolean =>
  err != null && typeof err === "object" && (err as any).name === "AbortError";

const abortError = () => new DOMException("Aborted", "AbortError");

/** sleep resolves after ms milliseconds, or rejects early if signal aborts. */
export const sleep = (
  ms: number,
  signal?: AbortSignal | null,
): Promise<void> =>
  new Promise((resolve, reject) => {
    if (signal != null && signal.aborted) {
      reject(abortError());
      return;
    }

    const onAbort = () => {
      clearTimeout(timer);
      reject(abortError());
    };

    const timer = setTimeout(() => {
      if (signal != null) {
        signal.removeEventListener("abort", onAbort);
      }
      resolve();
    }, ms);

    if (signal != null) {
      signal.addEventListener("abort", onAbort, { once: true });
    }
  });

/**
 * backoffDelay returns a "full jitter" exponential backoff delay.
 *
 * This ensures that clients retry actions with exponential backoff
 * including random jitter in order to avoid thundering herd problems.
 * 
 * This maxes out at `maxDelayMs` so that you can set the maximum
 * delay threshold to improve user experience.
 */
export const backoffDelay = (
  attempt: number,
  baseDelayMs: number = DEFAULT_BASE_DELAY_MS,
  maxDelayMs: number = DEFAULT_MAX_DELAY_MS,
): number => Math.random() * Math.min(maxDelayMs, baseDelayMs * Math.pow(attempt, 2));

/**
 * retryAfterMs parses a Retry-After header into milliseconds, in either the
 * delta-seconds or the HTTP-date form. Returns null when absent or unparseable.
 * 
 * Anubis doesn't currently return this header, but HTTP middleware in the critical
 * path may end up doing so. In order to be defensive it is better to just handle
 * this before it becomes an issue.
 */
const retryAfterMs = (response: Response): number | null => {
  const header = response.headers.get("Retry-After");
  if (header === null) {
    return null;
  }

  const seconds = Number(header);
  if (Number.isFinite(seconds)) {
    return Math.max(0, seconds * 1000);
  }

  const deadline = Date.parse(header);
  if (Number.isNaN(deadline)) {
    return null;
  }

  return Math.max(0, deadline - Date.now());
};

/**
 * fetchWithBackoff attempts to do a basic GET fetch of url, retrying
 * transient failures with jittered exponential backoff. It rejects with
 * the last error once attempts are exhausted, immediately on a
 * non-retryable status code, or with an AbortError if the cancellation
 * signal fires.
 */
export const fetchWithBackoff = async (
  url: string,
  options: BackoffOptions = {},
): Promise<Response> => {
  // populate default options
  const attempts = options.attempts ?? DEFAULT_ATTEMPTS;
  const baseDelayMs = options.baseDelayMs ?? DEFAULT_BASE_DELAY_MS;
  const maxDelayMs = options.maxDelayMs ?? DEFAULT_MAX_DELAY_MS;
  const signal = options.signal ?? null;

  let lastError: Error = new Error(`anubis: ${url} could not be fetched`);

  // If the server told us how long to wait, we wait for this number of
  // milliseconds instead of using the computed exponential backoff.
  let serverDelayMs: number | null = null;

  for (let attempt = 0; attempt < attempts; attempt++) {
    if (attempt > 0) {
      const delay =
        serverDelayMs !== null
          ? serverDelayMs
          : backoffDelay(attempt - 1, baseDelayMs, maxDelayMs);
      serverDelayMs = null;
      await sleep(delay, signal);
    }

    let response: Response;
    try {
      response = await fetch(url, signal === null ? {} : { signal });
    } catch (err) {
      if (isAbortError(err)) {
        throw err;
      }
      // If there is a network-level failure such as DNS resolution failure,
      // connection reset, or a server hang-up: this is the signature of an
      // overloaded server. Keep trying after backing off.
      lastError = err instanceof Error ? err : new Error(String(err));
      continue;
    }

    if (response.ok) {
      return response;
    }

    lastError = new Error(`anubis: ${url} returned HTTP ${response.status} (unretryable failure)`);

    if (RETRYABLE_STATUSES.indexOf(response.status) === -1) {
      throw lastError;
    }

    // XXX(Xe): Honor the server's Retry-After response header but use common
    // sense: a misconfigured (or actively hostile) upstream should not be
    // able to park the client for an hour or something ridiculous like that.
    const requested = retryAfterMs(response);
    if (requested !== null) {
      serverDelayMs = Math.min(requested, maxDelayMs);
    }
  }

  throw lastError;
};

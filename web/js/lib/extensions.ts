declare global {
  interface Window {
    // Work registered by challenge extension scripts.
    __anubisExtensions?: Promise<unknown>[];
  }
}

/**
 * Wait for challenge extensions to finish work and submit data to
 * the server. Gives up after timeoutMs.
 */
export async function waitForExtensions(timeoutMs: number = 5000): Promise<void> {
  const pending = window.__anubisExtensions ?? [];
  if (pending.length === 0) {
    return;
  }

  // XXX(Xe): Normally you'd use Promise.allSettled here, but that is
  // a Chrome 76 feature. CI tests down to Chrome 75 and for safety's
  // sake the JS is compiled with Chrome 66 in mind. As such, polyfill
  // Promise.allSettled by just mapping over the pending promises the
  // hard way.
  const settled = Promise.all(
    pending.map(p => Promise.resolve(p).catch(() => undefined))
  );

  let timer: ReturnType<typeof setTimeout> | undefined = undefined;
  const timeout = new Promise<void>((resolve) => {
    timer = setTimeout(resolve, timeoutMs);
  });

  await Promise.race([settled, timeout]);
  clearTimeout(timer);
}
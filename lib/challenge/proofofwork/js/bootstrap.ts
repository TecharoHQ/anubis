// Watch for `main.mjs` failing to load and re-injects it with jittered
// exponential backoff.
//
// Without this any failures loading `main.mjs` leaves the challenge page
// sitting on the "Loading..." message forever with no error messaging
// outside of the browser console, which is unavailable on mobile browsers.
//
// This is the worst way for Anubis to fail because it requires the user
// to manually reload the webpage, which fails the main design goal of
// the proof of work challenges: no user interaction is required to pass
// challenges. Paradoxically, this would increase the level of load to
// an already overwhelmed server, making the problem much worse.
//
// This is injected into the page as a raw constant embedded from this
// file instead of loaded dynamically so that things are more reliable.
//
// A few notes on the implementation of this and why it makes some weird
// assumptions about the browser environment:
//
// - Loading a module script uses a slightly different internal fetch path
//   than loading it as a normal javascript script. As such, if you try to
//   re-load _the exact same_ URL you end up getting a cached fetch
//   failure, which then amplifies the problem and makes the logic _never_
//   load. This is bad, so we have to inject extra URL parameters to the
//   Anubis server.
// - The canonical way to solve this in a `<script>` tag is the
//   `script.onerror` property, which sadly does not fire for every kind of
//   failure that is relevant to Anubis. As a result we have to use a
//   watchdog timer to ensure the `main.mjs` file actually booted.

import { backoffDelay } from "@lib/backoff";
import { g, h } from "@lib/xeact.mjs";

const getMany = (ids: string[]): (HTMLElement | null)[] => ids.map(g);

(() => {
  const tag = g("anubis-main");
  if (!tag) {
    console.debug("can't find anubis main.mjs element via ID `anubis-main`, bailing");
    return;
  }

  const src = tag.getAttribute("src");
  if (!src) {
    console.debug("can't find src attribute of script element `anubis-main`, bailing");
    return;
  }

  const joiner = src.indexOf("?") === -1 ? "?" : "&";
  const MAX_ATTEMPTS = 4;
  const BASE_DELAY_MS = 750;
  const MAX_DELAY_MS = 10000;
  // XXX(Xe): this timer is overly generous to account for mobile connections
  // and/or German train Wi-Fi.
  const WATCHDOG_MS = 10000;
  let attempt = 0;
  let generation = 0;

  const booted = () =>
    // @ts-ignore: global variable set by main.mjs
    (window as any).__anubisBooted === true;

  const fail = () => {
    const els = getMany(["anubis-script-error", "status", "progress"]);
    if (els.filter(x => x == null).length !== 0) {
      console.debug("missing one of the following elements: anubis-script-error, status, progress. cannot proceed, bailing.");
      return;
    }

    const [el, status, progress] = els as HTMLElement[];
    el.style.display = "block";
    status.style.display = "none";
    progress.style.display = "none";
  }

  const settleFor = (gen: number) =>
    () => {
      if (booted() || gen !== generation) {
        return;
      }

      generation++;
      attempt++;
      if (attempt > MAX_ATTEMPTS) {
        fail();
        return;
      }

      const delay = backoffDelay(attempt, BASE_DELAY_MS, MAX_DELAY_MS);
      setTimeout(inject, delay);
    };

  const inject = () => {
    if (booted()) {
      return;
    }

    const settle = settleFor(generation);
    const s = h("script", {
      async: true,
      type: "module",
      src: src + joiner + "anubisRetry=" + attempt,
      onerror: settle,
    });
    document.head.appendChild(s);
    setTimeout(settle, WATCHDOG_MS);
  };

  const initial = settleFor(0);
  tag.onerror = initial;
  setTimeout(initial, WATCHDOG_MS);
})();
import { execFile } from "node:child_process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

const LIGHTPANDA = "./var/lightpanda";
const TARGET = "http://localhost:8923/reqmeta";

// Anubis renders "Access Denied: error code <hash>" on its DENY page. The hash
// changes whenever the rule changes, so only match the stable prefix.
const DENIED = "Access Denied: error code";

async function lightpandaFetch(...extraArgs) {
  const { stdout } = await execFileAsync(
    LIGHTPANDA,
    ["fetch", "--dump", "html", ...extraArgs, TARGET],
    { maxBuffer: 16 * 1024 * 1024 },
  );
  return stdout;
}

// Sanity check: a request that isn't a headless browser must sail through. If
// this fails then Anubis isn't up yet or it's denying everything, and the
// Lightpanda assertions below would pass for the wrong reason.
async function checkControl() {
  const resp = await fetch(TARGET, {
    headers: { "User-Agent": "AnubisCI" },
  });

  if (resp.status !== 200) {
    throw new Error(`control request: wanted status 200, got ${resp.status}`);
  }
}

const cases = [
  {
    name: "default user agent",
    args: [],
  },
  {
    // Lightpanda still sends Sec-Ch-Ua when --user-agent is overridden, so
    // Anubis must catch it even though the User-Agent says otherwise.
    name: "spoofed user agent",
    args: ["--user-agent", "AnubisCI"],
  },
];

(async () => {
  await checkControl();

  let failed = false;

  for (const { name, args } of cases) {
    const page = await lightpandaFetch(...args);

    if (!page.includes(DENIED)) {
      console.log(page);
      console.log(`${name}: Lightpanda was not denied`);
      failed = true;
      continue;
    }

    console.log(`${name}: Lightpanda was denied`);
  }

  if (failed) {
    throw new Error("lightpanda smoke test failed");
  }

  process.exit(0);
})();

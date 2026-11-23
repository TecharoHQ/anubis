import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { after, before, test } from "node:test";
import { build } from "esbuild";
import { chromium } from "playwright";

const features = [
  "bulkMemory",
  "signExtensions",
  "mutableGlobals",
  "saturatedFloatToInt",
  "simd",
  "multiValue",
];
const emptyModule = Buffer.from([0, 97, 115, 109, 1, 0, 0, 0]);
let browser;
let server;
let origin;
let bundle;
let scenario;
let requests;

before(async () => {
  const result = await build({
    stdin: {
      contents:
        'import process from "./web/js/algorithms/wasm.ts"; window.solve = process;',
      resolveDir: process.cwd(),
    },
    bundle: true,
    write: false,
    plugins: [
      {
        name: "controlled-feature-probes",
        setup(build) {
          build.onResolve({ filter: /^wasm-feature-detect$/ }, () => ({
            path: "features",
            namespace: "test",
          }));
          build.onResolve({ filter: /^\.\/wasm2js$/ }, () => ({
            path: "wasm2js",
            namespace: "test",
          }));
          build.onLoad({ filter: /.*/, namespace: "test" }, ({ path }) => ({
            contents:
              path === "wasm2js"
                ? 'export default async () => ({hash: "wasm2js"});'
                : features
                    .map(
                      (name) =>
                        `export const ${name} = async () => { const value = window.features.${name}; if (value === "reject") throw new Error("probe failed"); return value ?? true; };`,
                    )
                    .join("\n"),
          }));
        },
      },
    ],
  });
  bundle = result.outputFiles[0].text;
  server = createServer(async (req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/") {
      res.setHeader("Content-Type", "text/html");
      res.end("<!doctype html><title>WASM regression</title>");
      return;
    }
    const wasmPath = url.pathname.match(
      /^\/prefix\/\.within\.website\/x\/cmd\/anubis\/static\/wasm\/(simd128|baseline|mvp)\/(sha256|hashx|argon2id)\.wasm$/,
    );
    if (wasmPath) {
      const [, tier, algorithm] = wasmPath;
      requests.push(tier);
      if (scenario.disconnect) {
        req.socket.destroy();
        return;
      }
      if (scenario.hang) {
        res.writeHead(200, { "Content-Type": "application/wasm" });
        res.flushHeaders();
        return;
      }
      if (scenario.network) {
        res.writeHead(404);
        res.end("missing");
        return;
      }
      res.setHeader("Content-Type", scenario.mime ?? "application/wasm");
      if (scenario.realWorker) {
        res.end(await readFile(`web/static/wasm/${tier}/${algorithm}.wasm`));
      } else {
        res.end(
          scenario.invalid?.includes(tier) ? "invalid wasm" : emptyModule,
        );
      }
      return;
    }
    if (url.pathname.endsWith("/worker/wasm.mjs")) {
      res.setHeader("Content-Type", "text/javascript");
      res.end(
        scenario.realWorker
          ? await readFile("web/static/js/worker/wasm.mjs")
          : `onmessage = ({ data }) => postMessage(${scenario.workerFailure ? '{error: "runtime failed"}' : '{hash: data.module instanceof WebAssembly.Module ? "compiled" : "bad module", nonce: 0, difficulty: data.difficulty}'});`,
      );
      return;
    }
    res.writeHead(404);
    res.end();
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  origin = `http://127.0.0.1:${server.address().port}`;
  browser = await chromium.launch();
});

after(async () => {
  await browser?.close();
  server?.closeAllConnections();
  await new Promise((resolve) => server?.close(resolve));
});

async function solve(config = {}) {
  scenario = config;
  requests = [];
  const page = await browser.newPage();
  try {
    await page.goto(origin);
    await page.evaluate((config) => {
      window.features = config.features ?? {};
      if (config.disconnect) Math.random = () => 0;
      if (config.unsupported) window.WebAssembly = undefined;
    }, config);
    await page.addScriptTag({ content: bundle });
    return await page.evaluate(async (config) => {
      const controller = new AbortController();
      const timeout = setTimeout(
        () => controller.abort(),
        config.abort ? 100 : 5000,
      );
      try {
        const result = await window.solve(
          {
            basePrefix: "/prefix",
            version: "regression",
            algorithm: config.algorithm ?? "sha256",
          },
          "00".repeat(32),
          1,
          controller.signal,
          undefined,
          1,
        );
        return { result };
      } catch (error) {
        return { error: { name: error.name, message: error.message } };
      } finally {
        clearTimeout(timeout);
      }
    }, config);
  } finally {
    await page.close();
  }
}

for (const [name, probes, expected] of [
  ["all features select SIMD", {}, "simd128"],
  ["missing SIMD selects baseline", { simd: false }, "baseline"],
  ["missing multivalue selects baseline", { multiValue: false }, "baseline"],
  ...features
    .slice(0, 4)
    .map((feature) => [
      `missing ${feature} selects MVP`,
      { [feature]: false },
      "mvp",
    ]),
  ...features.map((feature, index) => [
    `rejected ${feature} probe selects ${index < 4 ? "MVP" : "baseline"}`,
    { [feature]: "reject" },
    index < 4 ? "mvp" : "baseline",
  ]),
]) {
  test(name, async () => {
    assert.equal((await solve({ features: probes })).result?.hash, "compiled");
    assert.deepEqual(requests, [expected]);
  });
}

test("compile errors fall through SIMD and baseline to MVP", async () => {
  assert.equal(
    (await solve({ invalid: ["simd128", "baseline"] })).result?.hash,
    "compiled",
  );
  assert.deepEqual(requests, ["simd128", "baseline", "mvp"]);
});

test("baseline requirements gate SIMD even when SIMD is supported", async () => {
  assert.equal(
    (await solve({ features: { bulkMemory: false } })).result?.hash,
    "compiled",
  );
  assert.deepEqual(requests, ["mvp"]);
});

test("incorrect MIME retries buffered compilation within the same tier", async () => {
  assert.equal((await solve({ mime: "text/plain" })).result?.hash, "compiled");
  assert.deepEqual(requests, ["simd128"]);
});

test("HTTP failures do not select a lower tier", async () => {
  assert.match((await solve({ network: true })).error?.message, /HTTP 404/);
  assert.deepEqual(requests, ["simd128"]);
});

test("exhausted network retries stay within the selected tier", async () => {
  assert.equal((await solve({ disconnect: true })).error?.name, "TypeError");
  assert.ok(requests.length >= 5, "network fetches must retry");
  assert.ok(requests.every((tier) => tier === "simd128"));
});

test("aborted body fetch does not select a lower tier", async () => {
  assert.equal(
    (await solve({ hang: true, abort: true })).error?.name,
    "AbortError",
  );
  assert.deepEqual(requests, ["simd128"]);
});

test("worker failures do not select a lower tier", async () => {
  assert.match(
    (await solve({ workerFailure: true })).error?.message,
    /runtime failed/,
  );
  assert.deepEqual(requests, ["simd128"]);
});

test("final MVP compile failure reaches the caller", async () => {
  assert.equal(
    (await solve({ invalid: ["simd128", "baseline", "mvp"] })).error?.name,
    "CompileError",
  );
  assert.deepEqual(requests, ["simd128", "baseline", "mvp"]);
});

test("basic WASM failure keeps the wasm2js path", async () => {
  assert.equal((await solve({ unsupported: true })).result?.hash, "wasm2js");
  assert.deepEqual(requests, []);
});

test("generated MVP SHA256 solves through the actual WASM worker", async () => {
  const { result } = await solve({
    features: { bulkMemory: false },
    realWorker: true,
  });
  assert.deepEqual(requests, ["mvp"]);
  assert.ok(result, "challenge must finish");
  const nonce = Buffer.alloc(4);
  nonce.writeUInt32BE(result.nonce);
  const expected = createHash("sha256")
    .update(Buffer.alloc(32))
    .update(nonce)
    .digest("hex");
  assert.equal(result.hash, expected);
  assert.ok(parseInt(result.hash.slice(0, 2), 16) < 128);
});

for (const algorithm of ["hashx", "argon2id"]) {
  test(`generated MVP ${algorithm} proof passes baseline validation`, async () => {
    const { result } = await solve({
      algorithm,
      features: { bulkMemory: false },
      realWorker: true,
    });
    assert.deepEqual(requests, ["mvp"]);
    assert.ok(result, "challenge must finish");
    const { instance } = await WebAssembly.instantiate(
      await readFile(`web/static/wasm/baseline/${algorithm}.wasm`),
      { anubis: { anubis_update_nonce: () => {} } },
    );
    const wasm = instance.exports;
    const data = Buffer.alloc(32);
    new Uint8Array(wasm.memory.buffer, wasm.data_ptr(), data.length).set(data);
    wasm.set_data_length(data.length);
    const proof = Buffer.from(result.hash, "hex");
    assert.equal(proof.length, wasm.verification_hash_size());
    const verification = new Uint8Array(
      wasm.memory.buffer,
      wasm.verification_hash_ptr(),
      proof.length,
    );
    verification.set(proof);
    assert.equal(wasm.anubis_validate(result.nonce, 1), 1);
    // Validation can grow memory, so reacquire the view before corrupting it.
    const validatedProof = new Uint8Array(
      wasm.memory.buffer,
      wasm.verification_hash_ptr(),
      proof.length,
    );
    validatedProof[0] ^= 1;
    assert.equal(wasm.anubis_validate(result.nonce, 1), 0);
  });
}

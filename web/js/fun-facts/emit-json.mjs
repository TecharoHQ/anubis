// Emit per-language fun-fact JSON files from the TypeScript registry.
//
// This script is invoked by web/build.sh.  It bundles js/fun-facts/index.ts
// with esbuild into a temporary .mjs, dynamically imports it to read the
// `funFactsByLang` map, then writes one JSON file per language into the
// output directory.  The temporary module is deleted afterwards.
//
// Usage: node js/fun-facts/emit-json.mjs <output-dir>
import { mkdir, writeFile, rm } from "node:fs/promises";
import { resolve } from "node:path";
import { build } from "esbuild";

const outDir = resolve(process.argv[2] ?? "static/funfacts");
const entryPath = resolve("js/fun-facts/index.ts");
const tmpModule = resolve("static/fun-facts.index.tmp.mjs");

await build({
  entryPoints: [entryPath],
  bundle: true,
  format: "esm",
  platform: "node",
  target: "node18",
  outfile: tmpModule,
  write: true,
});

try {
  const mod = await import(`file://${tmpModule}`);
  const funFactsByLang = mod.funFactsByLang;

  if (!funFactsByLang || typeof funFactsByLang !== "object") {
    throw new Error("funFactsByLang not found in registry module");
  }

  await mkdir(outDir, { recursive: true });

  for (const [lang, facts] of Object.entries(funFactsByLang)) {
    if (!Array.isArray(facts) || facts.length === 0) {
      console.warn(`Skipping ${lang}: facts is empty or not an array`);
      continue;
    }
    const outFile = resolve(outDir, `${lang}.json`);
    await writeFile(outFile, JSON.stringify(facts) + "\n");
    console.log(`emitted funfacts/${lang}.json (${facts.length} facts)`);
  }
} finally {
  await rm(tmpModule, { force: true });
}

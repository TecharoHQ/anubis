#!/usr/bin/env node
// Sorts .vscode/project-words.txt alphabetically (case-insensitive) and
// removes duplicate/blank entries so cspell's dictionary stays tidy.
import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const target = join(here, "..", "project-words.txt");

const original = readFileSync(target, "utf8");

const words = [
  ...new Set(
    original
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line.length > 0),
  ),
];

words.sort((a, b) =>
  a.localeCompare(b, "en", { sensitivity: "base" }) ||
  a.localeCompare(b, "en"),
);

const sorted = words.join("\n") + "\n";

if (sorted === original) {
  console.log("project-words.txt already sorted");
  process.exit(0);
}

writeFileSync(target, sorted);
console.log(`sorted ${words.length} words in .vscode/project-words.txt`);

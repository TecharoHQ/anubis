// Registry of per-language fun facts.
//
// To add a new language:
//   1. Create <lang>.ts in this directory exporting `funFacts: string[]`.
//   2. Import it below and add it to the `funFactsByLang` map.
//   3. Run `npm run assets` — the build emits static/funfacts/<lang>.json.
import { funFacts as en } from "./en";

export const funFactsByLang: Record<string, string[]> = {
  en,
};

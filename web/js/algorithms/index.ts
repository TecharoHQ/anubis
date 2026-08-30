import fast from "./fast";
import wasm from "./wasm";

export default {
  fast: fast,
  slow: fast, // XXX(Xe): slow is deprecated, but keep this around in case anything goes bad
  // These names must match the ones lib/challenge/wasm registers server side.
  argon2id: wasm,
  hashx: wasm,
  sha256: wasm,
} as Record<string, any>;

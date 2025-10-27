import fast from "./fast";
import wasm from "./wasm";

export default {
  fast: fast,
  slow: fast, // XXX(Xe): slow is deprecated, but keep this around in case anything goes bad

  hashx: wasm,
  sha256: wasm,
}
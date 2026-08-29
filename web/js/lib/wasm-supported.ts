// https://stackoverflow.com/questions/47879864/how-can-i-check-if-a-browser-supports-webassembly
const isWASMSupported = (() => {
  try {
    if (typeof WebAssembly === "object"
      && typeof WebAssembly.instantiate === "function") {
      const module = new WebAssembly.Module(Uint8Array.of(0x0, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00));
      if (module instanceof WebAssembly.Module)
        return new WebAssembly.Instance(module) instanceof WebAssembly.Instance;
    }
  } catch (e) {
    return false;
  }
  return false;
})();

export default isWASMSupported;

export class WASMUnsupportedError extends Error {
  constructor() {
    super("WebAssembly is not supported in this browser. This is not a bug in Anubis. Please contact the system administrator for help.");
    this.name = "WASMUnsupportedError";
  }
}

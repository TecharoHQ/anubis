# Pre-built wasm2js binaries

The existience of this folder is a bit of a crime. Let me elaborate.

Anubis uses WebAssembly based checks so that it can have the proof of work run as _efficiently as possible_. This includes using the simd128 extension in WebAssembly so that clients take maximum possible advantage of their hardware. The goal of this is to make Anubis _go away as fast as it can_ so people don't see it for too long and complain.

However, many "privacy browsers", iOS Lockdown Mode, and other "enhanced web privacy" configuration guides tell people to disable WebAssembly. This would mean that clients that can't/won't execute WebAssembly are de-facto blocked from the service Anubis protects.

As a middle ground, the WebAssembly modules are compiled down to JavaScript a-la [Metal in The Birth and Death of JavaScript](https://www.destroyallsoftware.com/talks/the-birth-and-death-of-javascript). This will at least let the check logic run on client machines. It won't run efficiently as most of the configurations that disable WebAssembly also disable browser JIT engines, but it will _eventually_ finish.

The catch that makes this folder need to exist is that the tool that's being used for this, [wasm2js from binaryen](https://github.com/WebAssembly/binaryen), is packaged in distro repositories. The downside of the distro packaging is that it is _woefully_ out of date compared to what is required to actually compile the WebAssembly to JavaScript. This leaves us in a bit of a pickle.

The middle path here is to use WebAssembly to ship _the known working version_ of wasm2js in the git repo as well as the [build.sh](./build.sh) script used to build it. The build scripts will use this version of wasm2js as much as possible. These builds are known to be byte-for-byte reproducible as of [wasi-sdk v34.0-rc.2](https://github.com/WebAssembly/wasi-sdk/releases/tag/wasi-sdk-34-rc.2).

Hopefully this will be robust enough to survive reality, but this is the kind of thing you only learn the efficacy of the hard way.

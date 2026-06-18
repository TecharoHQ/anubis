# Pre-built wasm2js binaries

The existience of this folder is a bit of a crime. Let me elaborate.

Anubis uses WebAssembly based checks so that it can have the proof of work run as _efficiently as possible_. This includes using the simd128 extension in WebAssembly so that clients take maximum possible advantage of their hardware. The goal of this is to make Anubis _go away as fast as it can_ so people don't see it for too long and complain.

However, many "privacy browsers", iOS Lockdown Mode, and other "enhanced web privacy" configuration guides tell people to disable WebAssembly. This would mean that clients that can't/won't execute WebAssembly are de-facto blocked from the service Anubis protects.

As a middle ground, the WebAssembly modules are compiled down to JavaScript a-la [Metal in The Birth and Death of JavaScript](https://www.destroyallsoftware.com/talks/the-birth-and-death-of-javascript). This will at least let the check logic run on client machines. It won't run efficiently as most of the configurations that disable WebAssembly also disable browser JIT engines, but it will _eventually_ finish.

The catch that makes this folder need to exist is that the tool that's being used for this, [wasm2js from binaryen](https://github.com/WebAssembly/binaryen), is packaged in distro repositories. The downside of the distro packaging is that it is _woefully_ out of date compared to what is required to actually compile the WebAssembly to JavaScript. This leaves us in a bit of a pickle.

The middle path here is to use WebAssembly to ship _the known working version_ of wasm2js in the git repo as well as the [build.sh](./build.sh) script used to build it. The build scripts will use this version of wasm2js as much as possible. This version of wasm2js is known to be a reproducible binary at least on Linux.

Hopefully this will be robust enough to survive reality, but this is the kind of thing you only learn the efficacy of the hard way.

## Reproducibility

You'd think that if you have the same source code bytes, the same compiler code bytes, and the same compiler flags, all targeting the same target architecture, you'd be able to run your own build and get the same result I did, right?

Wrong. It's more complicated than that, but in ways that aren't really that obvious at the outset.

There are a shocking number of ways that you can create nondeterministic output in compilers. One of the easiest is by using the `__DATE__` and `__TIME__` macros:

```c++
// hello.cpp

#include <iostream>

int main() {
    std::cout << __DATE__ << " " << __TIME__ << std::endl;
    return 0;
}
```

Building and running it once gets me this:

```text
$ make clean && make hello.wasm && wasmtime run -W exceptions=y ./hello.wasm
rm -f hello.o hello.wasm
wasi-sdk-33.0-x86_64-linux/bin/wasm32-wasip1-clang++ -O3 -fwasm-exceptions -mllvm -wasm-use-legacy-eh=false  -c hello.cpp -o hello.o
wasi-sdk-33.0-x86_64-linux/bin/wasm32-wasip1-clang++ -O3 -fwasm-exceptions -mllvm -wasm-use-legacy-eh=false  -fwasm-exceptions -lunwind --no-wasm-opt hello.o -o hello.wasm
Jun 18 2026 00:00:59
```

Another time it gets me this:

```text
$ make clean && make hello.wasm && wasmtime run -W exceptions=y ./hello.wasm
rm -f hello.o hello.wasm
wasi-sdk-33.0-x86_64-linux/bin/wasm32-wasip1-clang++ -O3 -fwasm-exceptions -mllvm -wasm-use-legacy-eh=false  -c hello.cpp -o hello.o
wasi-sdk-33.0-x86_64-linux/bin/wasm32-wasip1-clang++ -O3 -fwasm-exceptions -mllvm -wasm-use-legacy-eh=false  -fwasm-exceptions -lunwind --no-wasm-opt hello.o -o hello.wasm
Jun 18 2026 00:01:11
```

Even though the source code had _the same bytes_, the output of the compiler was wildly different.

In order for you to trust the binaries I'm committing for wasm2js, I need to make sure that you can build the same bytes I build. To assure this, I'm taking the following steps:

- Pinning an exact version of wasi-sdk (v33).
- Pinning an exact version of binaryen (v130).
- Pinning the compile and link flags I used (see [`build.sh`](./build.sh)).
- Wiping the build tree before doing a compile.
- Stripping debug info (it's written nondeterministically).
- Stripping all non-required WASM sections (those are also likely written nondeterminstically).

Along the way of making this actually reproducible I ran into a few weird problems.

### Clang silently runs `wasm-opt` from `$PATH` behind your back

Among other tools like `wasm2js`, binaryen has [a bunch of other useful tools](https://github.com/WebAssembly/binaryen/tree/main) such as `wasm-opt`. `wasm-opt` optimizes WebAssembly compiler output to let you eke more performance out of your WebAssembly programs. This doesn't work in every circumstance, but when it does work it makes a _huge_ difference. As such, clang shells out to `wasm-opt` when doing builds.

This normally makes sense, but in this case it caused builds to fail on my DGX Spark because its version of `wasm-opt` is too old:

```text
$ uname -m && which wasm-opt && wasm-opt --version
aarch64
/usr/bin/wasm-opt
wasm-opt version 108
```

Compared to my tower which installs `wasm-opt` from [Homebrew](https://brew.sh):

```text
$ uname -m && which wasm-opt && wasm-opt --version
x86_64
/home/linuxbrew/.linuxbrew/bin/wasm-opt
wasm-opt version 130
```

Turns out that wasi-sdk and binaryen rely on the [WebAssembly Exceptions extension](https://github.com/WebAssembly/spec/blob/wasm-3.0/proposals/exception-handling/Exceptions.md). This is a reasonable thing to assume given that wasi-sdk mostly assumes you're building things for web browsers and [93.86% of browser users](https://caniuse.com/wf-wasm-exception-handling) have a browser engine new enough to support it.

Both wasmtime and wazero require you to flag into exception support. This is fine, we can just pass `-W exceptions=y` to wasmtime and use a custom runner harness for wazero. The annoying part is what happens when my arm machine's anemic build of wasm-opt sees exception handling instructions. It causes `wasm-opt` to exit because it doesn't understand them. This made the build fail.

The solution was to pass `--no-wasm-opt` at the linking step. This removed one angle of irreproducibility. I guess in the future we could make it use the version of `wasm-opt` it just built to optimize the output, but that may be a premature optimization for now.

### Clang relies on address layout for ordering things

The version of clang that I use to compile binaryen has some address-sensitive code generation hiding in its exception handling path. Raw pointer values leak into the order a handful of `try_table` blocks come out in. This surfaces as every build differing from the next by about 29 bytes:

```diff
-002a9af0: 2802 0441 0647 0d00 1f40 0103 0820 0241  (..A.G...@... .A
-002a9b00: 206a 2103 2002 4138 6a20 0141 086a 10b5   j!. .A8j .A.j..
-002a9b10: 8881 8000 2104 0b1f 4001 0304 2003 2004  ....!...@... . .
+002a9af0: 2802 0441 0647 0d00 1f40 0103 041f 4001  (..A.G...@....@.
+002a9b00: 0309 2002 4120 6a21 0320 0241 386a 2001  .. .A j!. .A8j .
+002a9b10: 4108 6a10 b588 8180 0021 040b 2003 2004  A.j......!.. . .
```

For those of you that can't read wasm bytecode from hex dumps like I can, here's it reformatted by instructions:

```diff
  i32.load  offset=4            ;; 28 02 04
  i32.const 6                   ;; 41 06
  i32.ne                        ;; 47
  br_if     0                   ;; 0d 00
- try_table (catch_all_ref 8)   ;; 1f 40 01 03 08
+ try_table (catch_all_ref 4)   ;; 1f 40 01 03 04
+ try_table (catch_all_ref 9)   ;; 1f 40 01 03 09
    local.get 2                 ;; 20 02
    i32.const 32                ;; 41 20
    i32.add                     ;; 6a
    local.set 3                 ;; 21 03
    local.get 2                 ;; 20 02
    i32.const 56                ;; 41 38
    i32.add                     ;; 6a
    local.get 1                 ;; 20 01
    i32.const 8                 ;; 41 08
    i32.add                     ;; 6a
    call 17461                  ;; 10 b5 88 81 80 00
    local.set 4                 ;; 21 04
  end                           ;; 0b
- try_table (catch_all_ref 4)   ;; 1f 40 01 03 04
    local.get 3                 ;; 20 03
    local.get 4                 ;; 20 04
```

It's nearly identical in terms of the computation, but the block orders are just different enough to also make the catch references differ. This also fires when you build this pinned version of wasm2js on arm64 machines.

To work around this, I took two steps:

1. Disable address-space randomization for this build using `setarch --addr-no-randomize`.
2. Create known good sha256 checksums for both x86_64 and arm64 via building this program on machines I trust.

I also made a CI job ensure this:

```yaml
- name: Ensure reproducibility
  run: |
    cd ./utils/wasm/wasm2js
    ./build.sh
    if sha256sum -c --status shasums.x86_64; then
      echo "OK: rebuilt modules match the recorded x86_64 checksums"
    elif sha256sum -c --status shasums.arm64; then
      echo "OK: rebuilt modules match the recorded arm64 checksums"
    else
      echo "::error::rebuilt wasm2js/wasm-opt match neither recorded checksum set on ${{ matrix.runner }}" >&2
      sha256sum wasm-opt_130.wasm wasm2js_130.wasm
      exit 1
    fi
```

To be extra sure, we have this job run on both x86_64 and arm64 hosts. I'd really love to have this be reproducible across hosts, but that's an upstream LLVM bug that I am not powerful enough to tackle. If you work on LLVM and are reading this, it would be nice to set a seed of some kind to ensure that this iteration order is fixed across architectures.

At the very least builds are deterministic _within_ architectures. This may be good enough.

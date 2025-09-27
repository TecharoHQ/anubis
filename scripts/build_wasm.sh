#!/usr/bin/env bash

mkdir -p ./web/static/wasm/{simd128,baseline}

cargo clean

# With simd128
RUSTFLAGS='-C target-feature=+simd128' cargo build --release --target wasm32-unknown-unknown
cp -vf ./target/wasm32-unknown-unknown/release/*.wasm ./web/static/wasm/simd128

cargo clean

# Without simd128
cargo build --release --target wasm32-unknown-unknown
cp -vf ./target/wasm32-unknown-unknown/release/*.wasm ./web/static/wasm/baseline
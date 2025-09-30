#!/usr/bin/env bash

set -euo pipefail

mkdir -p ./web/static/wasm/{simd128,baseline}

cargo clean

# With simd128
RUSTFLAGS='-C target-feature=+simd128' cargo build --release --target wasm32-unknown-unknown
cp -vf ./target/wasm32-unknown-unknown/release/*.wasm ./web/static/wasm/simd128

cargo clean

# Without simd128
cargo build --release --target wasm32-unknown-unknown
cp -vf ./target/wasm32-unknown-unknown/release/*.wasm ./web/static/wasm/baseline

for file in ./web/static/wasm/baseline/*.wasm; do
  echo $file
  rm -f ${file%.*}.wasmjs
  wasm2js $file -all -O4 --strip-debug --rse --rereloop --optimize-for-js --flatten --dce --dfo --fpcast-emu --denan --dealign --remove-imports --remove-unused-names --remove-unused-brs --reorder-functions --reorder-locals --strip-target-features --untee --vacuum -s 4 -ffm -lmu -tnh -iit -n -o ${file%.*}.mjs
  sed -i '1s$.*$const anubis_update_nonce = (_ignored) => { };$' ${file%.*}.mjs
done
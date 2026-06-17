#!/usr/bin/env bash

set -euo pipefail

src_dirs=(./wasm/pow ./wasm/anubis)
dst_dirs=(./web/static/wasm/simd128 ./web/static/wasm/baseline)

# Newest source file timestamp (unix seconds).
newest_src="$(find "${src_dirs[@]}" -type f -printf '%T@\n' | sort -n | tail -1)"

# Oldest destination file timestamp (unix seconds). Empty if no outputs exist yet
# (e.g. the output dirs haven't been created, in which case find would error out).
oldest_dst="$(find "${dst_dirs[@]}" -type f -name '*.wasm' -printf '%T@\n' 2>/dev/null | sort -n | head -1 || true)"

if [ -n "$oldest_dst" ] && awk "BEGIN { exit !($newest_src <= $oldest_dst) }"; then
	echo "wasm artifacts are up to date, skipping build"
	exit 0
fi

mkdir -p ./web/static/wasm/{simd128,baseline}

cargo clean

# With simd128
RUSTFLAGS='-C target-feature=+simd128' cargo build --release --target wasm32-unknown-unknown
cp -vf ./target/wasm32-unknown-unknown/release/*.wasm ./web/static/wasm/simd128

cargo clean

# Without simd128
cargo build --release --target wasm32-unknown-unknown
cp -vf ./target/wasm32-unknown-unknown/release/*.wasm ./web/static/wasm/baseline

cargo clean

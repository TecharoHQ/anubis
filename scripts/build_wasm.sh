#!/usr/bin/env bash

set -euo pipefail

src_dirs=(./wasm/pow ./wasm/anubis)
dst_dirs=(./web/static/wasm/simd128 ./web/static/wasm/baseline)

# rustc enables reference-types by default for wasm32-unknown-unknown. With it on,
# LLVM encodes the call_indirect table index as an overlong LEB128 (80 80 80 80 00)
# rather than the MVP's single reserved 0x00 byte. Engines predating the proposal
# read one byte, see 0x80, and refuse to compile the module:
#
#	CompileError: expected table index 0, found 128
#
# Turning the feature off with -Ctarget-feature is not enough because the shipped
# std rlibs are already encoded that way, so fixing it at the source would mean
# rebuilding std on nightly. Round-tripping through wasm-opt re-emits the MVP
# encoding instead. Keep this a pure decode/re-encode with no optimization passes:
# the wasm build of wasm-opt exhausts its stack on -O2, and we would rather not
# have an optimizer rewriting proof-of-work code.
WASM_OPT_FLAGS="--disable-reference-types"

run_wasm_opt() {
	if command -v wasm-opt 2>&1 >/dev/null; then
		wasm-opt $WASM_OPT_FLAGS "$@"
	elif command -v wasmtime 2>&1 >/dev/null; then
		wasmtime run -W exceptions=y --dir . ./utils/wasm/wasm2js/wasm-opt_130.wasm $WASM_OPT_FLAGS "$@"
	else
		go run ./utils/cmd/wazero-exec ./utils/wasm/wasm2js/wasm-opt_130.wasm $WASM_OPT_FLAGS "$@"
	fi
}

# Rewrite every module in a directory with the MVP call_indirect encoding.
mvp_encode() {
	local out
	for fname in "${1}"/*.wasm; do
		out="$(mktemp "${fname}.XXXXXX")"
		# wasm-opt warns that no passes were specified; the re-encode is the point.
		if ! run_wasm_opt "$fname" -o "$out" 2>/dev/null; then
			rm -f "$out"
			echo "wasm-opt failed on ${fname}" >&2
			exit 1
		fi
		mv -f "$out" "$fname"
	done
}

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

cargo clean --quiet

# With simd128
RUSTFLAGS='-C target-feature=+simd128' cargo build --quiet --release --target wasm32-unknown-unknown
cp -vf ./target/wasm32-unknown-unknown/release/*.wasm ./web/static/wasm/simd128
mvp_encode ./web/static/wasm/simd128

cargo clean --quiet

# Without simd128
cargo build --quiet --release --target wasm32-unknown-unknown
cp -vf ./target/wasm32-unknown-unknown/release/*.wasm ./web/static/wasm/baseline
mvp_encode ./web/static/wasm/baseline

cargo clean --quiet

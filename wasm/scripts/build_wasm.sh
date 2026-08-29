#!/usr/bin/env bash

set -euo pipefail

src_dirs=(./wasm/pow ./wasm/anubis)
dst_dirs=(./web/static/wasm/simd128 ./web/static/wasm/baseline)

# rustc enables reference-types, multivalue and friends by default for
# wasm32-unknown-unknown. With reference-types on, LLVM encodes the call_indirect
# table index as an overlong LEB128 (80 80 80 80 00) rather than the MVP's single
# reserved 0x00 byte. Engines predating the proposal read one byte, see 0x80, and
# refuse to compile the module:
#
#	CompileError: expected table index 0, found 128
#
# Clearing the features with -Ctarget-feature is not enough, because the shipped
# std rlibs are already encoded that way; fixing it at the source would mean
# rebuilding std on nightly. Instead each module is round-tripped through wasm-opt
# against the feature set that its oldest supported browser actually implements.
# That re-emits the MVP call_indirect encoding, and makes wasm-opt reject the
# module outright if it ever picks up a feature not on the list, so a toolchain
# upgrade becomes a loud build failure rather than a browser that cannot start.
#
# Keep this a pure decode/re-encode with no optimization passes: the wasm build of
# wasm-opt exhausts its stack on -O2, and we would rather not have an optimizer
# rewriting proof-of-work code.
#
# Chrome shipped sign-ext and mutable-globals in 74, bulk-memory and
# nontrapping-fptoint in 75, multivalue in 85 and simd in 91.
baseline_features="-mvp --enable-sign-ext --enable-mutable-globals --enable-bulk-memory --enable-nontrapping-float-to-int"
simd128_features="${baseline_features} --enable-multivalue --enable-simd"

run_wasm_opt() {
	if command -v wasm-opt 2>&1 >/dev/null; then
		wasm-opt "$@"
	elif command -v wasmtime 2>&1 >/dev/null; then
		wasmtime run -W exceptions=y --dir . ./utils/wasm/wasm2js/wasm-opt_130.wasm "$@"
	else
		go run ./utils/cmd/wazero-exec ./utils/wasm/wasm2js/wasm-opt_130.wasm "$@"
	fi
}

# reencode DIR FEATURE_FLAGS... rewrites every module in DIR against FEATURE_FLAGS.
reencode() {
	local dir="${1}"
	shift

	local out err
	for fname in "${dir}"/*.wasm; do
		out="$(mktemp "${fname}.XXXXXX")"
		err="$(mktemp)"
		# stderr is held back because wasm-opt warns on every module that no passes
		# were specified; the re-encode is the point. Show it if the module fails.
		if ! run_wasm_opt "$@" "${fname}" -o "${out}" 2>"${err}"; then
			cat "${err}" >&2
			rm -f "${out}" "${err}"
			echo "wasm-opt rejected ${fname}" >&2
			exit 1
		fi
		rm -f "${err}"
		mv -f "${out}" "${fname}"
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
reencode ./web/static/wasm/simd128 ${simd128_features}

cargo clean --quiet

# Without simd128
cargo build --quiet --release --target wasm32-unknown-unknown
cp -vf ./target/wasm32-unknown-unknown/release/*.wasm ./web/static/wasm/baseline
reencode ./web/static/wasm/baseline ${baseline_features}

cargo clean --quiet

#!/usr/bin/env bash

set -euo pipefail
shopt -s nullglob

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

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

# Which wasm-opt run_wasm_opt picked, for error messages.
wasm_opt_via=""

run_wasm_opt() {
	if command -v wasm-opt 2>&1 >/dev/null; then
		wasm_opt_via="wasm-opt from PATH"
		wasm-opt "$@"
	elif command -v wasmtime 2>&1 >/dev/null; then
		wasm_opt_via="wasm-opt_130.wasm under wasmtime"
		wasmtime run -W exceptions=y --dir . ./utils/wasm/wasm2js/wasm-opt_130.wasm "$@"
	elif node_runs_wasm; then
		wasm_opt_via="wasm-opt_130.wasm under node"
		node --no-warnings ./wasm/scripts/execer.mjs ./utils/wasm/wasm2js/wasm-opt_130.wasm "$@"
	else
		wasm_opt_via="wasm-opt_130.wasm under wazero-exec"
		go run ./utils/cmd/wazero-exec ./utils/wasm/wasm2js/wasm-opt_130.wasm "$@"
	fi
}

# The stderr file and the output file of the wasm-opt run in flight, empty when
# there is none.
held_stderr=""
held_out=""

# reencode holds wasm-opt's stderr back, so anything that kills this script in the
# middle of a run (set -u, set -e, a signal) would otherwise take the only
# explanation down with it. The trap runs with stderr still pointing at the held
# file, hence the copy of the real one on fd 3.
exec 3>&2
report_held_stderr() {
	local rc="$?"
	if [ -z "${held_stderr}" ]; then
		return
	fi
	if [ "${rc}" -ne 0 ]; then
		echo "build_wasm.sh exited with status ${rc} while running ${wasm_opt_via:-wasm-opt}, which points at a bug in this script rather than at the module" >&3
		if [ -s "${held_stderr}" ]; then
			echo "stderr up to that point:" >&3
			sed 's/^/    /' "${held_stderr}" >&3
		fi
	fi
	rm -f "${held_stderr}" "${held_out}"
}
trap report_held_stderr EXIT

copy_modules() {
	local src="${1}" dst="${2}"

	local files=("${src}"/*.wasm)
	if [ "${#files[@]}" -eq 0 ]; then
		echo "cargo built no modules in ${src}" >&2
		exit 1
	fi

	cp -vf "${files[@]}" "${dst}"
}

# reencode DIR FEATURE_FLAGS... rewrites every module in DIR against FEATURE_FLAGS.
reencode() {
	local dir="${1}"
	shift

	local files=("${dir}"/*.wasm)
	if [ "${#files[@]}" -eq 0 ]; then
		echo "no modules to re-encode in ${dir}" >&2
		exit 1
	fi

	for fname in "${files[@]}"; do
		held_out="$(mktemp "${fname}.XXXXXX")"
		held_stderr="$(mktemp)"
		# stderr is held back because wasm-opt warns on every module that no passes
		# were specified.
		if ! run_wasm_opt "$@" "${fname}" -o "${held_out}" 2>"${held_stderr}"; then
			echo "wasm-opt rejected ${fname}" >&2
			echo "  ran as: ${wasm_opt_via}" >&2
			echo "  flags:  $*" >&2
			echo "  stderr:" >&2
			sed 's/^/    /' "${held_stderr}" >&2
			echo "If it names a feature that is not enabled, the Rust toolchain now emits something outside that feature list. Read the comment above baseline_features in ${BASH_SOURCE[0]} before adding the flag, the list is what the oldest supported browsers implement." >&2
			rm -f "${held_out}" "${held_stderr}"
			held_stderr=""
			exit 1
		fi
		rm -f "${held_stderr}"
		held_stderr=""
		mv -f "${held_out}" "${fname}"
	done
}

# Newest source file timestamp (unix seconds).
newest_src="$(mtimes "${src_dirs[@]}" -type f | sort -n | tail -1)"

# Oldest destination file timestamp (unix seconds). Empty if no outputs exist yet
# (e.g. the output dirs haven't been created, in which case find would error out).
oldest_dst="$(mtimes "${dst_dirs[@]}" -type f -name '*.wasm' 2>/dev/null | sort -n | head -1 || true)"

if all_populated '*.wasm' "${dst_dirs[@]}" && [ -n "$oldest_dst" ] && awk "BEGIN { exit !($newest_src <= $oldest_dst) }"; then
	echo "wasm artifacts are up to date, skipping build"
	exit 0
fi

mkdir -p ./web/static/wasm/{simd128,baseline}

cargo clean --quiet

# With simd128
RUSTFLAGS='-C target-feature=+simd128' cargo build --quiet --release --target wasm32-unknown-unknown
copy_modules ./target/wasm32-unknown-unknown/release ./web/static/wasm/simd128
reencode ./web/static/wasm/simd128 ${simd128_features}

cargo clean --quiet

# Without simd128
cargo build --quiet --release --target wasm32-unknown-unknown
copy_modules ./target/wasm32-unknown-unknown/release ./web/static/wasm/baseline
reencode ./web/static/wasm/baseline ${baseline_features}

cargo clean --quiet

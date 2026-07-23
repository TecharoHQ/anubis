#!/usr/bin/env bash

set -euo pipefail

WASM2JS_VERSION="130"
WASM2JS_FLAGS="-all --strip-debug --rse --rereloop --optimize-for-js --flatten --dce --dfo --fpcast-emu --denan --dealign --remove-imports --remove-unused-names --remove-unused-brs --reorder-functions --reorder-locals --strip-target-features --untee --vacuum -s 4 -ffm -lmu -tnh -iit -n"
src_dirs=(./web/static/wasm/baseline)
dst_dirs=(./web/js/gen/wasm2js)

# Newest source file timestamp (unix seconds).
newest_src="$(find "${src_dirs[@]}" -type f -printf '%T@\n' | sort -n | tail -1)"

# Oldest destination file timestamp (unix seconds). Empty if no outputs exist yet
# (e.g. the output dirs haven't been created, in which case find would error out).
oldest_dst="$(find "${dst_dirs[@]}" -type f -printf '%T@\n' 2>/dev/null | sort -n | head -1 || true)"

if [ -n "$oldest_dst" ] && awk "BEGIN { exit !($newest_src <= $oldest_dst) }"; then
	echo "wasm2js artifacts are up to date, skipping build"
	exit 0
fi

run_wasm2js() {
	if command -v wasm2js 2>&1 >/dev/null; then
		echo ">> wasm2js (sys) ${*}"
		wasm2js $WASM2JS_FLAGS $*
	elif command -v wasmtime 2>&1 >/dev/null; then
		echo ">> wasm2js (wasmtime) ${*}"
		wasmtime run -W exceptions=y --dir . ./utils/wasm/wasm2js/wasm2js_130.wasm $WASM2JS_FLAGS $*
	else
		echo ">> wasm2js (wazero-exec, slow) ${*}"
		go run ./utils/cmd/wazero-exec ./utils/wasm/wasm2js/wasm2js_130.wasm $WASM2JS_FLAGS $*
	fi
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

mkdir -p ./web/js/gen/wasm2js

for fname in ./web/static/wasm/baseline/*.wasm; do
	output="./web/js/gen/wasm2js/$(basename $fname).js"
	run_wasm2js $fname -o "${output}"
	sed -i '1s$.*$const anubis = { anubis_update_nonce: (_ignored) => { } };$' $output
done

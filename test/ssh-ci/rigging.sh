#!/usr/bin/env bash

set -euo pipefail
[ ! -z "${DEBUG:-}" ] && set -x

if [ "$#" -ne 1 ]; then
	echo "Usage: rigging.sh <user@host>"
fi

declare -A Hosts

Hosts["riscv64"]="ubuntu@riscv64.techaro.lol" # GOARCH=riscv64 GOOS=linux
Hosts["ppc64le"]="ci@ppc64le.techaro.lol"     # GOARCH=ppc64le GOOS=linux
Hosts["aarch64-4k"]="rocky@192.168.2.52"      # GOARCH=arm64 GOOS=linux 4k page size
Hosts["aarch64-16k"]="ci@192.168.2.28"        # GOARCH=arm64 GOOS=linux 16k page size

CIRunnerImage="ghcr.io/techarohq/anubis/ci-runner:latest"
RunID=${GITHUB_RUN_ID:-$(uuidgen)}
RunFolder="anubis/runs/${RunID:?}"
Target="${Hosts["$1"]}"

# A build step that prints nothing for a few minutes leaves the connection idle
# for long enough that something between the runner and the host drops it, which
# kills the run with "client_loop: send disconnect: Broken pipe". Keepalives stop
# the connection from ever looking idle.
ssh() {
	command ssh -o ServerAliveInterval=30 -o ServerAliveCountMax=6 "$@"
}

ssh "${Target}" uname -av >/dev/null
ssh "${Target}" rm -rf "${RunFolder}"
ssh "${Target}" mkdir -p "${RunFolder}"
git archive HEAD | ssh "${Target}" tar xC "${RunFolder}"

# The WebAssembly artifacts are identical on every architecture and the target
# hosts are bad at building them: there is no fast WebAssembly runtime for
# wasm-opt and wasm2js on ppc64le or riscv64, and node dies with SIGILL on the
# riscv64 host. Ship whatever is already built here. tar keeps the timestamps,
# which are newer than the ones git archive gives the sources, so the build
# scripts on the target see them as up to date and skip the work.
PrebuiltGlobs=(
	web/static/wasm/simd128/*.wasm
	web/static/wasm/baseline/*.wasm
	web/js/gen/wasm2js/*.wasm.js
)
shopt -s nullglob
# shellcheck disable=SC2206 # the globs are meant to expand
Prebuilt=(${PrebuiltGlobs[@]})
shopt -u nullglob
if [ "${#Prebuilt[@]}" -ne 0 ]; then
	tar c "${Prebuilt[@]}" | ssh "${Target}" tar xC "${RunFolder}"
else
	echo "no prebuilt WebAssembly artifacts found, ${1} will build its own" >&2
fi

ssh "${Target}" <<EOF
  set -euo pipefail
  set -x
  mkdir -p anubis/cache/{go,go-build,node}
  podman pull ${CIRunnerImage}
  podman run --rm -it \
    -v "\$HOME/${RunFolder}:/app/anubis:z" \
    -v "\$HOME/anubis/cache/go:/root/go:z" \
    -v "\$HOME/anubis/cache/go-build:/root/.cache/go-build:z" \
    -v "\$HOME/anubis/cache/node:/root/.npm:z" \
    -w /app/anubis \
    ${CIRunnerImage} \
    sh /app/anubis/test/ssh-ci/in-container.sh
  rm -rf "\$HOME/${RunFolder}"
EOF

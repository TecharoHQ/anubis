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
RunFolder="anubis/runs/${RunID}"
Target="${Hosts["$1"]}"

ssh "${Target}" uname -av
ssh "${Target}" mkdir -p "${RunFolder}"
git archive HEAD | ssh "${Target}" tar xC "${RunFolder}"

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
  ssh "${Target}" rm -rf "${RunFolder}"
EOF

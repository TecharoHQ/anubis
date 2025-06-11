#!/usr/bin/env bash

set -euo pipefail
[ ! -z "${DEBUG:-}" ] && set -x

if [ "$#" -ne 1 ]; then
    echo "Usage: rigging.sh <user@host>"
fi

CIRunnerImage="ghcr.io/techarohq/anubis/ci-runner:latest"
RunID=${GITHUB_RUN_ID:-$(uuidgen)}
RunFolder="anubis/runs/${RunID}"
Target="${1}"

ssh "${Target}" uname -av
ssh "${Target}" mkdir -p "${RunFolder}"
git archive HEAD | ssh "${Target}" tar xC "${RunFolder}"

ssh "${Target}" << EOF
  set -euo pipefail
  set -x
  mkdir -p "anubis/cache/{go,go-build,node}"
  podman pull ${CIRunnerImage}
  podman run --rm -it \
    -v "\$HOME/${RunFolder}:/app/anubis" \
    -v "\$HOME/anubis/cache/go:/root/go" \
    -v "\$HOME/anubis/cache/go-build:/root/.cache/go-build" \
    -v "\$HOME/anubis/cache/node:/root/.npm" \
    -w /app/anubis \
    ${CIRunnerImage} \
    sh /app/anubis/test/ssh-ci/in-container.sh
  ssh "${Target}" rm -rf "${RunFolder}"
EOF
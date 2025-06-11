#!/usr/bin/env bash

set -euo pipefail
[ ! -z "${DEBUG:-}" ] && set -x

if [ "$#" -ne 1 ]; then
    echo "Usage: rigging.sh <user@host>"
fi

RunID=${GITHUB_RUN_ID:-$(uuidgen)}
RunFolder="anubis/runs/${RunID}"
Target="${1}"

ssh "${Target}" uname -av
ssh "${Target}" mkdir -p "${RunFolder}"
ssh "${Target}" mkdir -p "anubis/cache/{go,go-build,node}"
git archive HEAD | ssh "${Target}" tar xC "${RunFolder}"
ssh "${Target}" podman pull alpine:3.22
ssh "${Target}" podman run --rm -itv "\$HOME/${RunFolder}:/app/anubis" -v "\$HOME/anubis/cache/go:/root/go" -v "\$HOME/anubis/cache/go-build:/root/.cache/go-build" -v "\$HOME/anubis/cache/node:/root/.npm" -w /app/anubis alpine:3.22 sh /app/anubis/test/ssh-ci/in-container.sh
ssh "${Target}" rm -rf "${RunFolder}"
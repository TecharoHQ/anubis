#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "Usage: rigging.sh <user@host>"
fi

RunID=${$GITHUB_RUN_ID:-$(cat /proc/sys/kernel/random/uuid)}
RunFolder="anubis/${RunID}"
Target="${$1}.techaro.lol"

ssh "${Target}" uname -av
ssh "${Target}" mkdir -p "${RunFolder}"
git archive HEAD | ssh "${Target}" tar xC "${RunFolder}"
ssh "${Target}" podman pull alpine:3.22
ssh "${Target}" podman run --rm -itv "${RunFolder}:/app/anubis" -w /app/anubis alpine:3.22 sh /app/anubis/test/ssh-ci/in-container.sh
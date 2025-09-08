#!/usr/bin/env bash

export VERSION=$GITHUB_COMMIT-test
export KO_DOCKER_REPO=ko.local

function capture_vnc_snapshots() {
  sudo apt-get update && sudo apt-get install -y gvncviewer
  mkdir -p ./var
  while true; do
    timestamp=$(date +"%Y%m%d%H%M%S")
    gvnccapture localhost:0 ./var/snapshot_$timestamp.png 2>/dev/null
    sleep 1
  done
}

source ../../lib/lib.sh

if [ "$GITHUB_ACTIONS" = "true" ]; then
  capture_vnc_snapshots &
fi

set -euo pipefail

build_anubis_ko
mint_cert relayd

go run ../../cmd/cipra/ --compose-name $(basename $(pwd))

docker compose down -t 1 || :
docker compose rm -f || :

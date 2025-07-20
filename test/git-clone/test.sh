#!/usr/bin/env bash

set -eo pipefail

export VERSION=$GITHUB_COMMIT-test
export KO_DOCKER_REPO=ko.local

set -u

(
  cd ../.. && \
  ko build --platform=all --base-import-paths --tags="latest" --image-user=1000 --image-annotation="" --image-label="" ./cmd/anubis -L
)

rm -rf ./var/repos ./var/clones
mkdir -p ./var/repos ./var/clones

(cd ./var/repos && git clone --bare https://github.com/TecharoHQ/status.git)

docker compose up -d

sleep 2

(cd ./var/clones && git clone http://localhost:8005/status.git)

docker compose down
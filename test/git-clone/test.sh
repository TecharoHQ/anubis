#!/usr/bin/env bash

set -eo pipefail

export VERSION=$GITHUB_COMMIT-test
export KO_DOCKER_REPO=ko.local

set -u

source ../lib/lib.sh

build_anubis_ko

rm -rf ./var/repos ./var/clones
mkdir -p ./var/repos ./var/clones

(cd ./var/repos && git clone --bare https://github.com/TecharoHQ/status.git)

docker compose up -d

sleep 2

(cd ./var/clones && git clone http://localhost:8005/status.git)

exit 0
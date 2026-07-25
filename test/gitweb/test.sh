#!/usr/bin/env bash

set -eo pipefail

export VERSION=$GITHUB_COMMIT-test
export KO_DOCKER_REPO=ko.local

set -u

source ../lib/lib.sh

build_anubis_ko

docker compose up -d

backoff-retry node ./test.mjs

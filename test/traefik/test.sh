#!/usr/bin/env bash

set -eo pipefail

export VERSION=${GITHUB_SHA:-devel}-test
export KO_DOCKER_REPO=ko.local

set -u

source ../lib/lib.sh

build_anubis_ko

function cleanup() {
	docker compose down -t 1 || :
}

trap cleanup EXIT SIGINT

docker compose up -d

backoff-retry --try-count 20 node ./test.mjs

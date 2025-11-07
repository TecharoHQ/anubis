#!/usr/bin/env bash

set -euo pipefail

export VERSION=$GITHUB_COMMIT-test
export KO_DOCKER_REPO=ko.local

source ../lib/lib.sh

build_anubis_ko

function cleanup() {
	docker compose down
}

trap cleanup EXIT SIGINT

mint_cert registry.local.cetacean.club

docker compose up -d

backoff-retry skopeo \
	--insecure-policy \
	copy \
	--dest-tls-verify=false \
	docker://hello-world \
	docker://registry.local.cetacean.club:3004/hello-world

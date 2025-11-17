#!/usr/bin/env bash

set -eo pipefail

export VERSION=${GITHUB_SHA}-test
export KO_DOCKER_REPO=ko.local

set -u

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

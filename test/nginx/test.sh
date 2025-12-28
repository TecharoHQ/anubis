#!/usr/bin/env bash

export VERSION=$GITHUB_COMMIT-test
export KO_DOCKER_REPO=ko.local

source ../lib/lib.sh

set -euo pipefail

build_anubis_ko
mint_cert mimi.techaro.lol

docker run --rm \
	-v ./conf/nginx:/etc/nginx:ro \
	-v ../pki:/techaro/pki:ro \
	nginx \
	nginx -t

docker compose up -d

docker compose down -t 1 || :
docker compose rm -f || :

exit 0

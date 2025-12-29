#!/usr/bin/env bash

export VERSION=$GITHUB_COMMIT-test
export KO_DOCKER_REPO=ko.local

source ../lib/lib.sh

set -euo pipefail

mint_cert mimi.techaro.lol

docker run --rm \
	-v ./conf/nginx:/etc/nginx:ro \
	-v ../pki:/techaro/pki:ro \
	nginx \
	nginx -t

exit 0

#!/usr/bin/env bash

source ../lib/lib.sh

set -euo pipefail

mint_cert mimi.techaro.lol

docker run --rm \
	-v $PWD/conf/nginx:/etc/nginx:ro \
	-v $PWD/pki:/techaro/pki:ro \
	nginx \
	nginx -t

exit 0

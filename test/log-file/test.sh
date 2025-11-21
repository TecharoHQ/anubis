#!/usr/bin/env bash

set -euo pipefail

function cleanup() {
	pkill -P $$
}

trap cleanup EXIT SIGINT

# Build static assets
(cd ../.. && npm ci && npm run assets)

go tool anubis --help 2>/dev/null || :

go run ../cmd/httpdebug &

go tool anubis \
	--policy-fname ./anubis.yaml \
	--use-remote-address \
	--target=http://localhost:3923 &

sleep 2

backoff-retry node ./test.mjs

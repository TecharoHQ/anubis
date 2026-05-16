#!/usr/bin/env bash

set -euo pipefail

function cleanup() {
	pkill -P $$
}

trap cleanup EXIT SIGINT

go tool anubis --help 2>/dev/null || :

go run ../cmd/unixhttpd &

go tool anubis \
	--policy-fname ./anubis.yaml \
	--use-remote-address \
	--target=unix://$(pwd)/unixhttpd.sock &

go tool backoff-retry node ./test.mjs

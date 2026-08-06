#!/usr/bin/env bash

set -euo pipefail

function cleanup() {
	pkill -P $$
}

trap cleanup EXIT SIGINT

# Build static assets
(cd ../.. && npm ci && npm run assets)

mkdir -p ./var

# Lightpanda only ships nightly builds, so grab one if we don't have it yet.
if [ ! -x ./var/lightpanda ]; then
	curl -fsSL -o ./var/lightpanda \
		"https://github.com/lightpanda-io/browser/releases/download/nightly/lightpanda-$(uname -m)-linux"
	chmod a+x ./var/lightpanda
fi

go tool anubis --help 2>/dev/null || :

go run ../cmd/unixhttpd &

go tool anubis \
	--policy-fname ./anubis.yaml \
	--use-remote-address \
	--target=unix://$(pwd)/unixhttpd.sock &

backoff-retry node ./test.mjs

#!/usr/bin/env bash

set -euo pipefail

source ../lib/lib.sh

mint_cert "relayd.local.cetacean.club"

# Build static assets
(cd ../.. && npm ci && npm run assets)

# Spawn three jobs:

# HTTP daemon that listens over a unix socket (implicitly ./unixhttpd.sock)
go run ../cmd/unixhttpd &

# A copy of Anubis, specifically for the current Git checkout
go tool anubis \
  --bind=./anubis.sock \
  --bind-network=unix \
  --socket-mode=0700 \
  --policy-fname=../anubis_configs/aggressive_403.yaml \
  --target=unix://$(pwd)/unixhttpd.sock &

# A simple TLS terminator that forwards to Anubis, which will forward to
# unixhttpd
go run ../cmd/relayd \
  --proxy-to=unix://./anubis.sock \
  --cert-dir=../pki/relayd.local.cetacean.club &

export NODE_TLS_REJECT_UNAUTHORIZED=0

backoff-retry node test.mjs

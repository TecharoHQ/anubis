#!/usr/bin/env bash

set -euo pipefail

# Remove lingering .sock files, relayd and unixhttpd will do that too but
# measure twice, cut once.
rm *.sock ||:

# If the transient local TLS certificate doesn't exist, mint a new one
if [ ! -f ../pki/relayd.local.cetacean.club/cert.pem ]; then
  # Subshell to contain the directory change
  (
    cd ../pki \
    && mkdir -p relayd.local.cetacean.club \
    && \
    # Try using https://github.com/FiloSottile/mkcert for better DevEx,
    # but fall back to using https://github.com/jsha/minica in case
    # you don't have that installed.
    (
      mkcert \
        --cert-file ./relayd.local.cetacean.club/cert.pem \
        --key-file ./relayd.local.cetacean.club/key.pem relayd.local.cetacean.club \
      || go tool minica -domains relayd.local.cetacean.club
    )
  )
fi

# Build static assets
(cd ../.. && npm ci && npm run assets)

# Spawn three jobs:

# HTTP daemon that listens over a unix socket (implicitly ./unixhttpd.sock)
go run ../cmd/unixhttpd &

# A copy of Anubis, specifically for the current Git checkout
go tool anubis \
  --bind=./anubis.sock \
  --bind-network=unix \
  --policy-fname=../anubis_configs/aggressive_403.yaml \
  --target=unix://$(pwd)/unixhttpd.sock &

# A simple TLS terminator that forwards to Anubis, which will forward to
# unixhttpd
go run ../cmd/relayd \
  --proxy-to=unix://./anubis.sock \
  --cert-dir=../pki/relayd.local.cetacean.club &

# When you press control c, kill all the child processes to clean things up
trap 'echo signal received!; kill $(jobs -p); wait' SIGINT SIGTERM

echo "open https://relayd.local.cetacean.club:3004/reqmeta"

# Wait for all child processes to exit
wait

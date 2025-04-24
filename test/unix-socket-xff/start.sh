#!/usr/bin/env bash

set -euo pipefail

rm *.sock ||:

if [ ! -f ../pki/relayd.local.cetacean.club/cert.pem ]; then
  (
    cd ../pki \
    && mkdir -p relayd.local.cetacean.club \
    && \
    (
      mkcert --cert-file ./relayd.local.cetacean.club/cert.pem --key-file ./relayd.local.cetacean.club/key.pem relayd.local.cetacean.club \
      || go tool minica -domains relayd.local.cetacean.club
    )
  )
fi

(cd ../.. && npm ci && npm run assets)

go run ../cmd/unixhttpd &
go tool anubis \
  --bind=./anubis.sock \
  --bind-network=unix \
  --target=unix://$(pwd)/unixhttpd.sock &
go run ../cmd/relayd \
  --proxy-to=unix://./anubis.sock \
  --cert-dir=../pki/relayd.local.cetacean.club &

trap 'echo signal received!; kill $(jobs -p); wait' SIGINT SIGTERM

echo "open https://relayd.local.cetacean.club:3004/reqmeta"

wait

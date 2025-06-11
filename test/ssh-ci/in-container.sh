#!/usr/bin/env sh

set -euo pipefail

apk add -U go nodejs git build-base git npm bash zstd brotli gzip
npm ci
npm run build
SKIP_INTEGRATION=1 go test ./...
#!/usr/bin/env sh

set -euo pipefail
set -x

npm ci
npm run build
SKIP_INTEGRATION=1 go test ./...
go tool yeet --force-git-version=v$(cat VERSION)
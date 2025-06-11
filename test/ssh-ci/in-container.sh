#!/usr/bin/env sh

set -euo pipefail

npm ci
npm run build
SKIP_INTEGRATION=1 go test ./...
go tool yeet --force-git-version=$(cat VERSION)
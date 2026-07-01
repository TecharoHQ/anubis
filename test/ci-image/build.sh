#!/usr/bin/env bash

set -euo pipefail
set -x

version=$(curl -sX GET "https://api.github.com/repos/actions/runner/releases/latest" | jq --raw-output '.tag_name')
version="${version#*v}"
version="${version#*release-}"

docker buildx build --platform=linux/arm64,linux/amd64 --build-arg VERSION=${version} -t ghcr.io/techarohq/anubis/ci-image:latest .
docker push ghcr.io/techarohq/anubis/ci-image:latest

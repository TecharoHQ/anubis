#!/usr/bin/env bash

source ../lib/lib.sh

export KO_DOCKER_REPO=ko.local

set -euo pipefail

# Step 1: Config validation
mint_cert haproxy-simple.test

# Combine cert and key for HAProxy SSL directory format
cat pki/haproxy-simple.test/cert.pem pki/haproxy-simple.test/key.pem >pki/haproxy-simple.test/haproxy.pem

docker run --rm \
	-v $PWD/conf/haproxy:/usr/local/etc/haproxy:ro \
	-v $PWD/pki:/etc/techaro/pki:ro \
	haproxytech/haproxy-alpine:3.0 \
	haproxy -c -f /usr/local/etc/haproxy/haproxy.cfg

# Step 2: Runtime testing
echo "Starting services..."
docker compose up -d

sleep 5

echo "Services are healthy. Starting runtime tests..."
export NODE_TLS_REJECT_UNAUTHORIZED=0
node test.mjs

# Cleanup happens automatically via trap in lib.sh

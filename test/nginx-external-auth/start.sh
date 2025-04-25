#!/usr/bin/env bash

set -euo pipefail

# Build container image
(
  cd ../.. \
  && npm ci \
  && npm run container -- \
      --docker-repo ttl.sh/techaro/anubis-external-auth \
      --docker-tags ttl.sh/techaro/anubis-external-auth:latest
)

kubectl apply -k .
echo "open https://nginx.local.cetacean.club, press control c when done"

control_c() {
  kubectl delete -k .
  exit
}
trap control_c SIGINT

sleep infinity
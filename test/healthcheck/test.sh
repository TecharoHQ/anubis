#!/usr/bin/env bash

set -eo pipefail

export VERSION=$GITHUB_COMMIT-test
export KO_DOCKER_REPO=ko.local

set -u

(
  cd ../.. && \
  docker buildx bake
)


docker compose up -d

attempt=1
max_attempts=5
delay=2

while ! docker compose ps | grep healthy; do
  if (( attempt >= max_attempts )); then
    echo "Service did not become healthy after $max_attempts attempts."
    exit 1
  fi
  echo "Waiting for healthy service... attempt $attempt"
  sleep $delay
  delay=$(( delay * 2 ))
  attempt=$(( attempt + 1 ))
done

docker compose down
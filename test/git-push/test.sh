#!/usr/bin/env bash

set -eo pipefail

export VERSION=$GITHUB_COMMIT-test
export KO_DOCKER_REPO=ko.local

set -u

source ../lib/lib.sh

build_anubis_ko

rm -rf ./var/repos ./var/foo
mkdir -p ./var/repos

(cd ./var/repos && git init --bare foo.git && cd foo.git && git config http.receivepack true)

docker compose up -d

sleep 2

(
  cd var && \
  mkdir foo && \
  cd foo && \
  git init && \
  touch README && \
  git add . && \
  git config user.name "Anubis CI" && \
  git config user.email "social+anubis-ci@techaro.lol" && \
  git commit -sm "initial commit" && \
  git push -u http://localhost:3000/git/foo.git master
)

exit 0
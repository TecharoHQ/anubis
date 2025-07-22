#!/usr/bin/env bash

set -x

set -euo pipefail

source ../lib/lib.sh

build_anubis_ko

docker compose up -d --build

export NODE_TLS_REJECT_UNAUTHORIZED=0

sleep 2

backoff-retry node test.mjs

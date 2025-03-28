#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"

esbuild js/main.mjs --sourcemap --bundle --minify --outfile=static/js/main.mjs
gzip -f -k static/js/main.mjs
zstd -f -k --ultra -22 static/js/main.mjs
brotli -fZk static/js/main.mjs
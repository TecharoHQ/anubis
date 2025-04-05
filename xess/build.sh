#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"
npx postcss ./xess.css -o xess.min.css
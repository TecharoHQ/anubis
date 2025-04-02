#!/bin/sh

set -euo pipefail

cd "$(dirname "$0")"
postcss ./xess.css -o xess.min.css

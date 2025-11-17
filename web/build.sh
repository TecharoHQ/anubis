#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"

LICENSE='/*
@licstart  The following is the entire license notice for the
JavaScript code in this page.

Copyright (c) 2025 Xe Iaso <xe.iaso@techaro.lol>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

Includes code from https://github.com/aws/aws-sdk-js-crypto-helpers which is
used under the terms of the Apache 2 license.

@licend  The above is the entire license notice
for the JavaScript code in this page.
*/'

# Copy localization files to static directory
mkdir -p static/locales
cp ../lib/localization/locales/*.json static/locales/

shopt -s nullglob globstar

for file in js/**/*.ts js/**/*.mjs; do
  out="static/${file}"
  if [[ "$file" == *.ts ]]; then
    out="static/${file%.ts}.mjs"
  fi

  mkdir -p "$(dirname "$out")"

  esbuild "$file" --sourcemap --bundle --minify --outfile="$out" --banner:js="$LICENSE"
  gzip -f -k -n "$out"
  zstd -f -k --ultra -22 "$out"
  brotli -fZk "$out"
done

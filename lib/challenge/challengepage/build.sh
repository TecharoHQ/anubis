#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"

LICENSE='/*
@licstart  The following is the entire license notice for the
JavaScript code in this page.

Copyright (c) 2026 Xe Iaso <xe.iaso@techaro.lol>

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

@licend  The above is the entire license notice
for the JavaScript code in this page.

## Rationale

This script catches `main.mjs` failing to load and re-injects it with
jittered exponential backoff.

Without this any failures loading `main.mjs` leaves the challenge page
sitting on the "Loading..." message forever with no error messaging
outside of the browser console, which is unavailable on mobile browsers.

Disable loading this script at your peril. You have been warned.
*/'

mkdir -p static/js

shopt -s nullglob globstar

for file in js/**/*.ts js/**/*.mjs; do
	# Mirrors web/build.sh: js/lib/ is for shared modules, not entry points.
	if [[ "$file" == js/lib/* ]]; then
		continue
	fi

	out="static/${file}"
	if [[ "$file" == *.ts ]]; then
		out="static/${file%.ts}.mjs"
	fi

	mkdir -p "$(dirname "$out")"

	esbuild "$file" --bundle --minify --target=chrome66 --outfile="$out" --banner:js="$LICENSE"
done

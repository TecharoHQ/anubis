#!/usr/bin/env bash

set -euo pipefail

go install within.website/x/cmd/vmbully@v1.31.1-0.20261010045245-5388b6218199
vmbully script --user ci test.js

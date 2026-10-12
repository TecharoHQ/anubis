#!/usr/bin/env bash

set -euo pipefail

go install within.website/x/cmd/vmbully@v1.31.1-0.20261012190235-f9a7c830a701
vmbully script --os-template local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst test.js

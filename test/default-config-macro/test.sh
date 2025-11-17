#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"
python3 -c 'import yaml'
python3 ./compare_bots.py
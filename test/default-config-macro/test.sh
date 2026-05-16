#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"

python3 -m venv .env
source .env/bin/activate
pip install pyyaml

python3 -c 'import yaml'
python3 ./compare_bots.py

#!/usr/bin/env bash

sudo chown -R vscode:vscode ./node_modules

npm ci &
go mod download &
go install ./utils/cmd/... &
go install mvdan.cc/sh/v3/cmd/shfmt@latest &

wait

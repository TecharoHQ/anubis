#!/usr/bin/env bash

pwd

npm ci &
go mod download &
go install ./utils/cmd/... &

wait

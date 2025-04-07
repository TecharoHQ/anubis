NODE_MODULES = node_modules
VERSION := $(shell cat ./VERSION)

.PHONY: build assets deps lint prebaked-build test

assets:
	npm run assets

deps:
	npm ci
	go mod download

build: deps
	npm run build
	@echo "Anubis is now built to ./var/anubis"

all: build

lint:
	go vet ./...
	go tool staticcheck ./...

prebaked-build:
	go build -o ./var/anubis -ldflags "-X 'github.com/TecharoHQ/anubis.Version=$(VERSION)'" ./cmd/anubis

test:
	npm run test
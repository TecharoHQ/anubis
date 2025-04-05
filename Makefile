NODE_MODULES = node_modules
VERSION := $(shell cat ./VERSION)

.PHONY: build assets deps lint prebaked-build test

$(NODE_MODULES):
	npm run assets

assets: $(NODE_MODULES)

deps: assets
	npm ci
	go mod download

build: deps
	npm run build
	@echo "Anubis is now built to ./var/anubis"

all: build

lint:
	go vet ./...
	staticcheck ./...

prebaked-build:
	go build -o ./var/anubis -ldflags "-X 'github.com/TecharoHQ/anubis.Version=$(VERSION)'" ./cmd/anubis

test:
	npm run test
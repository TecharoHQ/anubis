VERSION= $(shell cat ./VERSION)
GO?= go
NPM?= npm

.PHONY: build assets deps lint prebaked-build test

all: build

deps:
	$(NPM) ci
	$(GO) mod download

assets: deps
	$(GO) generate ./...
	./web/build.sh
	./xess/build.sh

build: assets
	$(GO) build -o ./var/anubis ./cmd/anubis
	@echo "Anubis is now built to ./var/anubis"

lint: assets
	$(GO) vet ./...
	$(GO) tool staticcheck ./...

prebaked-build:
	$(GO) build -o ./var/anubis -ldflags "-X 'github.com/TecharoHQ/anubis.Version=$(VERSION)'" ./cmd/anubis

test: assets
	$(GO) test ./...

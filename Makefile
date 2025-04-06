NODE_MODULES = node_modules

.PHONY: build assets deps lint test

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

test:
	npm run test
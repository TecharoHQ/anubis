all: build

.PHONY: build assets deps lint test

node_modules: package.json package-lock.json
	npm ci

assets: node_modules
	npm run assets

deps: assets
	go mod download

build: deps
	npm run build
	@echo "Anubis is now built to ./var/anubis"

lint:
	go vet ./...
	staticcheck ./...

test:
	npm run test
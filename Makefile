NODE_MODULES = node_modules

.PHONY: build assets deps lint test

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

test:
	npm run test
VERSION= $(shell cat ./VERSION)
GO?= go
NPM?= npm

.PHONY: build assets deps lint prebaked-build test test-certs

all: build

deps:
	$(NPM) ci
	$(GO) mod download

assets: PATH:=$(PWD)/node_modules/.bin:$(PATH)
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
	$(GO) tool govulncheck ./...

prebaked-build:
	$(GO) build -o ./var/anubis -ldflags "-X 'github.com/TecharoHQ/anubis.Version=$(VERSION)'" ./cmd/anubis

test-certs:
	@command -v mkcert >/dev/null 2>&1 || { echo "mkcert is not installed. Please install it: brew install mkcert"; exit 1; }
	@mkdir -p internal/test/certs
	@if [ ! -f internal/test/certs/localhost+2.pem ] || [ ! -f internal/test/certs/localhost+2-key.pem ]; then \
		echo "Generating localhost certificates for tests..."; \
		cd internal/test/certs && mkcert localhost 127.0.0.1 ::1; \
	else \
		echo "Test certificates already exist"; \
	fi
	@echo "Installing mkcert root CA..."
	@mkcert -install 2>/dev/null || echo "Warning: failed to install mkcert root CA (may already be installed)"

test: assets test-certs
	$(GO) test ./...

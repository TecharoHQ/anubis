---
title: Local development
---

:::note

TL;DR: `npm ci && npm run dev`

:::

Anubis requires the following tools to be installed to do local development:

- [Go](https://go.dev) - the programming language that Anubis is written in
- [esbuild](https://esbuild.github.io/) - the JavaScript bundler Anubis uses for its production JS assets
- [Node.JS & NPM](https://nodejs.org/en) - manages some build dependencies
- `gzip` - compresses production JS (part of coreutils)
- `zstd` - compresses production JS
- `brotli` - compresses production JS

If you have [Homebrew](https://brew.sh) installed, you can install all the dependencies with one command:

```text
brew bundle
```

If you don't, you may need to figure out equivalents to the packages in Homebrew.

## Running Anubis locally

```text
npm run dev
```

Or to do it manually:

- Run `npm run assets` every time you change the CSS/JavaScript
- `go run ./cmd/anubis` with any CLI flags you want

## Building JS/CSS assets

```text
npm run assets
```

If you change the build process, make sure to update `build.sh` accordingly.

## Production-ready builds

```text
npm run container
```

This builds a prod-ready container image with [ko](https://ko.build). If you want to change where the container image is pushed, you need to use environment variables:

```text
DOCKER_REPO=registry.host/org/repo DOCKER_METADATA_OUTPUT_TAGS=registry.host/org/repo:latest npm run container
```

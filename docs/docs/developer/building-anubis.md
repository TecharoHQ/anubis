---
title: Building Anubis without Docker
---

:::note

These instructions may work, but for right now they are informative for downstream packagers more than they are ready-made instructions for administrators wanting to run Anubis on their servers.

:::end

## Tools needed

In order to build a production-ready binary of Anubis, you need the following packages in your environment:

- [Go](https://go.dev) - the programming language that Anubis is written in
- [esbuild](https://esbuild.github.io/) - the JavaScript bundler Anubis uses for its production JS assets
- [Node.JS & NPM](https://nodejs.org/en) - manages some build dependencies
- `gzip` - compresses production JS (part of coreutils)
- `zstd` - compresses production JS
- `brotli` - compresses production JS

## Install dependencies

```text
go mod download
npm ci
```

## Building static assets

```text
npm run assets
```

## Building Anubis to the `./var` folder

```text
go build -o ./var/anubis ./cmd/anubis
```

From this point it is up to you to make sure that `./var/anubis` ends up in the right place. You may want to consult the `./run` folder for useful files such as a systemd unit and `anubis.env.default` file.

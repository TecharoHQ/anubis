---
title: Building Anubis without Docker
---

:::note

These instructions may work, but for right now they are informative for downstream packagers more than they are ready-made instructions for administrators wanting to run Anubis on their servers.

:::

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
make deps
```

This will download Go and NPM dependencies.

## Building static assets

```text
make assets
```

This will build all static assets (CSS, JavaScript) for distribution.

## Building Anubis to the `./var` folder

```text
make build
```

From this point it is up to you to make sure that `./var/anubis` ends up in the right place. You may want to consult the `./run` folder for useful files such as a systemd unit and `anubis.env.default` file.

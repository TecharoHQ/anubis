---
title: Building Anubis without Docker
---

:::note

These instructions may work, but for right now they are informative for downstream packagers more than they are ready-made instructions for administrators wanting to run Anubis on their servers. Pre-made binary package support is being tracked in [#156](https://github.com/TecharoHQ/anubis/issues/156).

:::

## Entirely from source

If you are doing a build entirely from source, here's what you need to do:

:::info

If you maintain a package for Anubis v1.15.x or older, you will need to update your package build. You may want to use one of the half-baked tarballs if your distro/environment of choice makes it difficult to use npm.

:::

### Tools needed

In order to build a production-ready binary of Anubis, you need the following packages in your environment:

- [Go](https://go.dev) at least version 1.24 - the programming language that Anubis is written in
- [esbuild](https://esbuild.github.io/) - the JavaScript bundler Anubis uses for its production JS assets
- [Node.JS & NPM](https://nodejs.org/en) - manages some build dependencies
- `gzip` - compresses production JS (part of coreutils)
- `zstd` - compresses production JS
- `brotli` - compresses production JS

To upgrade your version of Go without system package manager support, install `golang.org/dl/go1.24.2` (this can be done from any version of Go):

```text
go install golang.org/dl/go1.24.2@latest
go1.24.2 download
```

### Install dependencies

```text
make deps
```

This will download Go and NPM dependencies.

### Building static assets

```text
make assets
```

This will build all static assets (CSS, JavaScript) for distribution.

### Building Anubis to the `./var` folder

```text
make build
```

From this point it is up to you to make sure that `./var/anubis` and `./var/robots2policy` end up in
the right place. You may want to consult the `./run` folder for useful files such as a systemd unit
and `anubis.env.default` file.

## "Pre-baked" tarball

The `anubis-src-with-vendor` tarball has many pre-build steps already done, including:

- Go module dependencies are present in `./vendor`
- Static assets (JS, CSS, etc.) are already built in CI

This means you do not have to manage Go, NPM, or other ecosystem dependencies.

When using this tarball, all you need to do is build `./cmd/anubis`:

```text
make prebaked-build
```

Anubis will be built to `./var/anubis` and the robots2policy tool to `./var/robots2policy`.

## Development dependencies

Optionally, you can install the following dependencies for development:

- [Staticcheck](https://staticcheck.dev/docs/getting-started/) (optional, not required due to [`go tool staticcheck`](https://www.alexedwards.net/blog/how-to-manage-tool-dependencies-in-go-1.24-plus), but required if you are using any version of Go older than 1.24)

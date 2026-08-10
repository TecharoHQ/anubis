---
title: Packaging Anubis
---

Anubis is a web application written in Go with components in JavaScript. Packaging it should be similar to other applications with the same constraints.

:::note

Please do not sidestep the build logic shipped by Anubis. We assume that the build logic shipped by Anubis is run. Any deviations from the semantics of this build logic may result in bugs that we will be blamed for. Please make our lives easier by not reinventing the wheel.

:::

## Entirely from source

In order to build a production-ready binary of Anubis, you need the following packages in your environment:

- [Go](https://go.dev) at least version 1.26
- [esbuild](https://esbuild.github.io/)
- [Node.JS & NPM](https://nodejs.org/en) (latest LTS)
- `gzip`
- `zstd`
- `brotli`

Anubis does not require a C compiler to build, meaning it is fully compatible with cross-compilation.

:::note

To upgrade your version of Go without system package manager support, install `golang.org/dl/go1.26.4` (this can be done from any version of Go):

```text
go install golang.org/dl/go1.26.4@latest
go1.26.4 download
```

:::

### Install dependencies

```text
npm ci
go mod download
```

This will download Go and NPM dependencies.

### Building static assets

```text
npm run assets
```

This will build all static assets (CSS, JavaScript) for distribution.

### Building Anubis to the `./var` folder

```text
npm run build
```

From this point it is up to you to make sure that `./var/anubis` and `./var/robots2policy` end up in the right place. You may want to consult the `./run` folder for useful files such as a systemd unit and the `anubis.env.default` file. The default configuration file is in `./data/botPolicies.yaml`.

## "Pre-baked" tarball

The `anubis-src-with-vendor` tarball has many pre-build steps already done, including:

- Go module dependencies are present in `./vendor`
- Static assets (JS, CSS, etc.) are already built in CI

This means you do not have to manage Go, NPM, or other ecosystem dependencies.

When using this tarball, all you need to do is build `./cmd/anubis`:

```text
make prebaked-build
```

Anubis will be built to `./var/anubis` and the robots2policy tool to `./var/robots2policy`. Install these via your normal packaging process.

## Development dependencies

Optionally, you can install the following dependencies for development:

- [Staticcheck](https://staticcheck.dev/docs/getting-started/) (optional, not required due to [`go tool staticcheck`](https://www.alexedwards.net/blog/how-to-manage-tool-dependencies-in-go-1.24-plus), but required if you are using any version of Go older than 1.24)

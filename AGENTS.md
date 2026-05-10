# Agent instructions

Primary agent documentation is in `CONTRIBUTING.md`. You MUST read this file before proceeding.

## Useful Commands

```shell
npm ci           # install node dependencies
npm run assets   # build JS/CSS (required before any Go build/test)
npm run build    # assets + go build -> ./var/anubis
npm run dev      # assets + run locally with --use-remote-address
```

## Testing

```shell
npm run test
```

## Linting

```shell
go vet ./...
go tool staticcheck ./...
go tool govulncheck ./...
```

## Commit Messages

Commit messages follow the [**Conventional Commits**](https://www.conventionalcommits.org/en/v1.0.0/) format:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`

- Add `!` after type/scope for breaking changes or include `BREAKING CHANGE:` in the footer.
- Keep descriptions concise, imperative, lowercase, and without a trailing period.
- Reference issues/PRs in the footer when applicable.
- **ALL git commits MUST be made with `--signoff`.** This is mandatory.

### Attribution Requirements

AI agents must disclose what tool and model they are using in the "Assisted-by" commit footer:

```text
Assisted-by: [Model Name] via [Tool Name]
```

Example:

```text
Assisted-by: GLM 4.6 via Claude Code
```

## PR Checklist

- Add description of changes to `[Unreleased]` in `docs/docs/CHANGELOG.md`.
- Add test cases for bug fixes and behavior changes.
- Run integration tests: `npm run test:integration`.
- All commits must have verified (signed) signatures.

## Key Conventions

- **Security-first**: This is security software. Code reviews are strict. Always add tests for bug fixes. Consider adversarial inputs.
- **Configuration**: YAML-based policy files. Config structs validate via `Valid() error` methods returning sentinel errors.
- **Store interface**: `lib/store.Interface` abstracts key-value storage.
- **Environment variables**: Parsed from flags via `flagenv`. Use `.env` files locally (loaded by `godotenv/autoload`). Never commit `.env` files.
- **Assets must be built first**: JS/CSS assets are embedded into the Go binary. Always run `npm run assets` before `go test` or `go build`.
- **CEL expressions**: Policy rules support CEL (Common Expression Language) expressions for advanced matching. See `lib/policy/expressions/`.
- **Confirmed Caddy plugin workflow**: `plugins/caddy` is a root-module package. Verify it with `xcaddy build --with github.com/TecharoHQ/anubis/plugins/caddy` style builds; generated runtime assets under `lib/challenge/preact/static/`, `web/static/js/`, and `web/static/locales/` must stay tracked because Go module consumers compile and serve embedded challenge assets without running generators first.
- **Confirmed Caddy dependency floor**: When adding Caddy v2.11.2, keep `golang.org/x/net >= v0.53.0` and `github.com/smallstep/certificates >= v0.30.0`; `go tool govulncheck ./...` reported reachable findings with the older `x/net v0.51.0` and `smallstep/certificates v0.30.0-rc3` graph.
- **Confirmed Caddy package-global guard**: `plugins/caddy` uses a mutex-backed `withAnubisGlobals` helper around provision/serve because Anubis templates still read `anubis.BasePrefix`, `PublicUrl`, `ForcedLanguage`, and `UseSimplifiedExplanation` package globals. Keep this guard unless the core library is refactored to pass those settings through request/server state.
- **Confirmed Caddy global defaults**: `plugins/caddy` registers `anubis` as both a Caddyfile global option and handler directive. A global `{ anubis { ... } }` block supplies Caddyfile defaults only; handler-level `anubis` blocks merge those defaults before validation and still provision independent Anubis servers per route. Caddyfile parsing tracks explicitly set fields so local zero values and optional boolean `false` values override global defaults.
- **Confirmed Caddy live middleware path**: A Caddy v2.11.2 binary built with `xcaddy --with github.com/TecharoHQ/anubis/plugins/caddy --with github.com/TecharoHQ/anubis=.` served a request through `anubis` into a downstream Caddy `respond` handler when using `lib/testdata/permissive.yaml` and `X-Real-Ip: 10.0.0.1`.
- **Confirmed Caddy remote address behavior**: With `use_remote_addr`, `plugins/caddy` populates `X-Real-Ip` from the request remote address only when that header is missing; an existing `X-Real-Ip` value is preserved for trusted proxy setups.
- **Confirmed Caddy matcher syntax**: `RegisterHandlerDirective` extracts an optional matcher before `plugins/caddy` parses its config. Use `policy_file <path>` inside the block for policy files, especially absolute paths that could otherwise look like Caddy path matchers.
- **Confirmed Caddy review risk checks**: As of 2026-05-10, `plugins/caddy` targets current Caddy v2.11.2, uses Caddy's logger, parses Caddyfile numbers/booleans with `strconv`, and orders `anubis` after `templates`; these directly cover public review concerns on competing Caddy PRs.
- **Confirmed Caddy placeholder and cleanup behavior**: `plugins/caddy` expands Caddy `{env.*}` and `{file.*}` placeholders for string settings during `Provision`, and implements `Cleanup` to cancel policy contexts and close policy stores that expose a `Close` method on Caddy reload.
- **Confirmed Caddy race check**: `GOTOOLCHAIN=go1.26.3 go test -race ./plugins/caddy` passes with the current `withAnubisGlobals` guard and non-parallel plugin tests.
- **Confirmed nested test module tidy**: After root Go/Caddy dependency changes, also run `cd test && go mod tidy` and `cd test && go test ./...`; the nested module has its own `go.mod` and builds utility commands against the replaced root module.
- **Confirmed package build version fallback**: `go tool yeet` package metadata must receive a semver-compatible version. On non-tag PR commits, `git describe` output is not semver and yeet returns `devel`; `yeetfile.js` now falls back to `<package.json version>-dev` for package metadata, binary ldflags, and source tarball `VERSION` files.

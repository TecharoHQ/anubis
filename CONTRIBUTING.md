# Contributing to Anubis

Anubis is a Web AI Firewall Utility (WAIFU) written in Go. It uses sha256 proof-of-work challenges to protect upstream HTTP resources from scraper bots. This is security software -- correctness matters.

## Build & Run

Prerequisites: Go 1.24+, Node.js (any supported version), esbuild, gzip, zstd, brotli. Install all with `brew bundle` if you are using Homebrew.

```shell
npm ci           # install node dependencies
npm run assets   # build JS/CSS (required before any Go build/test)
npm run build    # assets + go build -> ./var/anubis
npm run dev      # assets + run locally with --use-remote-address
```

## Testing

```shell
# Run all unit tests (assets must be built first)
npm run test              # or: make test

# Run a single test by name
go test -run TestClampIP ./internal/

# Run a single test file's package
go test ./lib/config/

# Run tests with verbose output
go test -v -run TestBotValid ./lib/config/
```

### Smoke tests

The `tests` folder contains "smoke tests" that are intended to set up Anubis in production-adjacent settings and testing it against real infrastructure tools. A smoke test is a folder with `test.sh` that sets up infrastructure, validates the behaviour, and then tears it down. Smoke tests are run in GitHub actions with `.github/workflows/smoke-tests.yaml`.

## Linting

```shell
go vet ./...
go tool staticcheck ./...
go tool govulncheck ./...
```

## Code Generation

The project uses `go generate` for templ templates and stringer. Always run `npm run generate` (or `make assets`) before building or testing. Generated files include:

- `web/*.templ` -> templ-generated Go code
- `web/static/` -> bundled/minified JS and CSS (with .gz, .zst, .br variants)

## Project Layout

Important folders:

- `cmd/anubis`: Main entrypoint for the project. This is the program that runs on servers.
- `lib/*`: The core library for Anubis and all of its features. This is internal code that is made public for ease of downstream consumption. No API stability is guaranteed. Use at your own risk.
- `internal/*`: Actual internal code that is private to the implementation of Anubis. If you need to use a package in this, please copy it out and manually vendor it in your own project.
- `test/*` Smoke tests (see dedicated section for details).
- `web`: Frontend HTML templates.
- `xess`: Frontend CSS framework and build logic.

## Code Style

### Go

This project follows the idioms of the Go standard library. Generally follow the patterns that upstream Go uses, including:

- Prefer packages from the standard library unless there is no other option.
- Use package import aliases only when package names collide.
- Use `goimports` to format code. Run with `npm run format`.
- Use sentinel errors as package-level variables prefixed with `Err` (such as `ErrBotMustHaveName`). Wrap with `fmt.Errorf("package: small message giving context: %w", err)`.
- Use `log/slog` for structured logging. Pass loggers as arguments to functions. Use `lg.With` to preload with context. Prefer using `slog.Debug` unless you absolutely need to report messages to users, some users have magical thinking about log verbosity.
- Name PublicFunctionsAndTypes in PascalCase. Name privateFunctionsAndTypes in camelCase.
- Acronyms stay uppercase (`URL`, `HTTP`, `IP`, `DNS`, etc.)
- Enumerations should use strong types with validation logic for parsing remote input.
- Be conservative in what you send but liberal in what you accept.
- Anything reading configuration values should use both `json` and `yaml` struct tags. Use pointer values for optional configuration values.
- Use [table-driven tests](https://go.dev/wiki/TableDrivenTests) when writing test code.
- Use [`t.Helper()`](https://pkg.go.dev/testing#T.Helper) in helper code (setup/teardown scaffolding).
- Use [`t.Cleanup()`](https://pkg.go.dev/testing#T.Cleanup) to tear down per-test or per-suite scaffolding.
- Use [`errors.Is`](https://pkg.go.dev/errors#Is) for validating function results against sentinel errors.
- Prefer same-package tests over black-box tests (`_test` packages).

### JavaScript / TypeScript

- Source lives in `web/js/`. Built with esbuild, bundled and minified.
- Uses Preact (not React).
- No linter config. Keep functions small. Use `const` by default.

### Templ Templates

Anubis uses [Templ](https://templ.guide) for generating HTML on the server.

- `.templ` files in `web/` generate Go code. Run `go generate ./...` (or `npm run assets`) after modifying them.
- Templates receive typed Go parameters. Keep logic in Go, not templates.

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

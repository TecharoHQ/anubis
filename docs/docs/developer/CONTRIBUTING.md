# Contributing to Anubis

Anubis is a Web AI Firewall Utility (WAIFU) written in Go. This is security software, correctness and test coverage matters.

## Build & Run

Prerequisites:

- Go 1.26 or later.
- Node.js (any supported LTS version).
- esbuild, gzip, zstd, brotli, msitools

If you have [Homebrew](https://brew.sh) installed: install all development tools at once with `brew bundle`.

### Useful commands

```shell
npm ci           # install source-level dependencies
npm run assets   # build JS/CSS (required before any Go build/test)
npm run build    # assets + go build -> ./var/anubis
npm run dev      # assets + run locally with --use-remote-address
npm run format   # format all code to project standards
```

When using `npm run dev`, Anubis runs on `http://localhost:8923` and points to `http://localhost:3000`. The documentation site listens on port `3000` by default.

## Testing

```shell
# Run all unit tests (assets must be built first)
npm run test

# Run integration tests with real browsers
npm run test:integration

# Run a single test by name
go test -run TestClampIP ./internal/

# Run a single test file's package
go test ./lib/config/

# Run tests with verbose output
go test -v -run TestBotValid ./lib/config/
```

### Smoke tests

Anubis has functionality tests called smoke tests. Read more about them in the [smoke testing page](./smoke-tests.mdx).

## Linting

```shell
go vet ./...
go tool staticcheck ./...
go tool govulncheck ./...
```

## Code Generation

Anubis makes heavy use of code generation with `go generate` for HTML templating with [templ](https://templ.guide/), CSS optimization, and JavaScript building.

Run all code generation logic with `npm run assets`.

## Project Layout

Important folders:

- `cmd/anubis`: Main entrypoint for the project. Parses command line flags and starts listening over HTTP.
- `lib/*`: The core library for Anubis and all of its features. This is internal code that is made public for ease of downstream consumption by [BotStopper](../admin/botstopper.mdx).<br/><br/>No API stability is guaranteed. Use at your own risk.
- `internal/*`: Actual internal code that is private to the implementation of Anubis. If you need to use a package in this, please copy it out and manually vendor it in your own project.
- `test/*`: [Smoke tests](./smoke-tests.mdx).
- `web`: Frontend HTML templates.
- `xess`: Frontend CSS framework and build logic.

## Code Style

Anubis is written in several programming languages. Please keep these guidelines in mind:

### Go

This project follows standard Go idioms. These include:

- The standard library is already a dependency. Use the standard library as much as possible.
- Avoid package import aliases unless you have no other choice.
- Use `go tool goimports` to format code. Run with `npm run format`.
- Use sentinel errors as package-level variables prefixed with `Err` (such as `ErrImportIsInvalid`). Wrap errors with [`fmt.Errorf`](https://pkg.go.dev/fmt#Errorf)'s `%w` formatting verb. Compare against sentinel errors in tests with `errors.Is`.
- Use [`log/slog`](https://pkg.go.dev/log/slog) for all logging. Use the local logger variable named `lg` or create it from the `Server` instance. Preload context with [`lg.With`](https://pkg.go.dev/log/slog#With).
- Pass logger instances to functions as a variable right after the context argument.
- Do not log messages with a level above `Debug` unless the condition being logged is important for administrators to know.
- Administrators have configured fail2ban pipelines on Anubis' logging output. Keep this in mind when changing any aspect of any log line.
- Be conservative in what you send but liberal in what you accept.
- Use [table-driven tests](https://go.dev/wiki/TableDrivenTests) when writing test code.
- Use [`t.Helper()`](https://pkg.go.dev/testing#T.Helper) in helper code (setup/teardown scaffolding).
- Use [`t.Cleanup()`](https://pkg.go.dev/testing#T.Cleanup) to tear down per-test or per-suite scaffolding.

### Configuration parsing

The configuration file is the main interface administrators have into Anubis. Keep these things in mind when adding features to the configuration file:

- Return all configuration errors at once from validators (`Valid() error` methods that return many errors at once with `errors.Join`) so administrators can fix all their configuration problems at once.
- Enumerations should use strong types with validation logic for parsing remote input.
- All configuration values should have both `json` and `yaml` struct tags set.
- Use pointer values for optional configuration values.

### TypeScript

Please follow these guidelines:

- Prefer writing all front-end code in TypeScript. It is mostly there to ensure that development is done correctly.
- Try to avoid JavaScript features newer than Chrome 75 unless you have a polyfill for backwards compatibility.
- Keep functions small.
- Use `const` by default and `let` if values need to change.
- Make the code as unambiguous as possible.

### HTML Templates

Anubis uses [templ](https://templ.guide) for generating HTML on the server.

- Running `go generate` or `npm run assets` must regenerate HTML templates.
- Templates receive typed Go parameters.
- Templates are for presentation only. Any business logic belongs in Go, not templ.

## Commit Messages

Commit messages follow the [**Conventional Commits**](https://www.conventionalcommits.org/en/v1.0.0/) format:

```text
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Types**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`

- Avoid breaking changes when possible. If you must introduce a breaking change, add `!` after the type/scope and include `BREAKING CHANGE:` with the reason for the breaking change in the bottom of your commit message.
- Keep descriptions concise, imperative, lowercase, and without trailing punctuation.
- All git commits must include a [Developer Certificate of Origin](https://en.wikipedia.org/wiki/Developer_Certificate_of_Origin). TL;DR: `git commit --signoff`.

Mark commits as closing or fixing issues in trailers. For example:

```text
Closes: #1234
Fixes: #1234
Replaces: #1234
Ref: TecharoHQ/yeet#4
```

## PR Checklist

When you are ready to send your contribution as a pull request, please do the following:

- Add description of changes to the `[Unreleased]` section of `docs/docs/CHANGELOG.md`.
- Add test cases for bug fixes and behavior changes.
- Add a [smoke test](./smoke-tests.mdx) if applicable.
- Run integration tests: `npm run test:integration`.
- All commits must have [verified (signed) signatures](./signed-commits.md).

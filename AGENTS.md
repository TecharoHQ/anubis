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
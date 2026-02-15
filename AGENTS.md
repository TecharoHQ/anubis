# Agent instructions

Primary agent documentation is in `CONTRIBUTING.md`. You MUST read this file before proceeding.

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

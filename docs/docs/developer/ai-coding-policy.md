# AI Coding Policy

The Anubis repository has `AGENTS.md` and `CLAUDE.md` files in it. These files are there to point AI agent tools at the [contribution guidelines](./CONTRIBUTING.md) and to add the following text to context windows:

## Attribution Requirements

AI agents must disclose what tool and model they are using in the "Assisted-by" commit footer:

```text
Assisted-by: [Model Name] via [Tool Name]
```

Example:

```text
Assisted-by: GLM 4.6 via Claude Code
```

This is here intentionally so that low-effort AI agent use is detected. This makes AI agents tattle on themselves.

## Guidelines

If your use of AI agent tools is largely respectful of the fact that maintainers have limited time for review, you will likely not run into issues.

We kindly ask the following:

- Pull requests that contain commits with this metadata attached will have their reviews be deprioritized to make room for human authored work.
- People that abuse the system with a flood of low-quality contributions ("AI slop") will be banned upon their first infraction.
- Please understand the actions and implications of every contribution you make, regardless of what tools were used in its creation.
- Please do not put direct AI agent output in documentation or error messages.
- If you really are going to submit an AI-authored PR, please make sure that your code has sufficient test coverage. Look into skills such as [go-table-driven-tests](https://www.skills.sh/tigrisdata/skills/go-table-driven-tests).

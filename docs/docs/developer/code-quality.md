---
title: Code quality guidelines
---

When submitting code to Anubis, please take the time to consider the fact that this project is security software. If things go bad, bots can pummel sites into oblivion. This is not ideal for uptime.

As such, code reviews will be a bit more strict than you have seen in other projects. This is not people trying to be mean, this is a side effect of taking the problem seriously.

When making code changes, try to do the following:

- If you're submitting a bugfix, add a test case for it
- If you're changing the JavaScript, make sure the integration tests pass (`npm run test:integration`)

## Commit messages

Anubis follows the Go project's conventions for commit messages. In general, an ideal commit message should read like this:

```text
path/to/folder: brief description of the change

If the change is subtle, has implementation consequences, or is otherwise
not entirely self-describing: take the time to spell out why. If things
are very subtle, please also amend the documentation accordingly
```

The subject of a commit message should be the second half of the sentence "This commit changes the Anubis project to:". Here's a few examples:

- `disable DroneBL by default`
- `port the challenge to WebAssembly`

The extended commit message is also your place to give rationale for a new feature. When maintainers are reviewing your code, they will use this to figure out if the burden from feature maintainership is worth the merge.

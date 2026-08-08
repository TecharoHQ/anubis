# Functional and smoke testing for Anubis

This directory contains all the functional testing infrastructure for Anubis.

## Smoke tests

A smoke test is a folder with an executable file named `test.sh`. This `test.sh` file is responsible for starting up Anubis, running some logic [ideally with `backoff-retry`](../utils/cmd/backoff-retry/), and then returning 0 for success or anything else for failure. CI will `backoff-retry` smoke tests to ensure they aren't flaky.

### Best practices for smoke tests

- Source `lib/lib.sh`. It is there to save you time.
- If you are starting services with `docker compose`, use `build_anubis_ko` and load anubis from `ko.local/anubis`.
- Make tests as deterministic as possible.
- Use the default configuration as much as possible.
- Make tests run as fast as possible.
- If you need to do anything involving reading response metadata, write your logic in JavaScript and put it in `test.mjs`.
- If your smoke test exists because of an event, horrifying discovery, or other tale of woe, record it in `README.md` as a tale of woe. See [the gitweb smoke test](./gitweb/README.md) for an example.

Make sure to add smoke tests to [the smoke testing GitHub Actions workflow](../.github/workflows/smoke-tests.yml).

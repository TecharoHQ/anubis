# Anubis sweep policies

Each `*.yaml` here is a full `botPolicies` ruleset. Every `.yaml` file results
in a new chromesweep test by [Gubal](https://github.com/TecharoHQ/gubal).

To add a pass: drop a new `*.yaml` file in this directory and rebuild. No code
change is needed.

Constraints:

- The filename (without `.yaml`) becomes a Kubernetes ConfigMap name
  (`anubis-policy-<name>`), so use DNS-safe names: lowercase letters, digits, `-`.
- Rulesets must **CHALLENGE** browser user-agents, not `ALLOW` them: the smoke Job
  asserts the Anubis challenge page (which contains "Anubis") is served. An
  `ALLOW`ed request is proxied to the backend, whose body lacks "Anubis", and the
  pre-check would fail for reasons unrelated to the browser.
- A ruleset that Anubis rejects makes its pod crashloop; that policy's rollout
  times out and every version under it is reported as `error`.

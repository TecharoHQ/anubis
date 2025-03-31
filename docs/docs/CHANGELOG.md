---
sidebar_position: 999
---

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

- Added a periodic cleanup routine for the decaymap that removes expired entries, ensuring stale data is properly pruned.
- Added a no-store Cache-Control header to the challenge page
- Hide the directory listings for Anubis' internal static content
- Changed `--debug-x-real-ip-default` to `--use-remote-address`, getting the IP address from the request's socket address instead.
- DroneBL lookups have been disabled by default
- Static asset builds are now done on demand instead of the results being committed to source control
- The Dockerfile has been removed as it is no longer in use
- Developer documentation has been added to the docs site
- Show more errors when some predictable challenge page errors happen ([#150](https://github.com/TecharoHQ/anubis/issues/150))
- Verification page now shows hash rate and a progress bar for completion probability.
- Added the `--debug-benchmark-js` flag for testing proof-of-work performance during development.
- Use `TrimSuffix` instead of `TrimRight` on containerbuild
- Fix the startup logs to correctly show the address and port the server is listening on
- Add [LibreJS](https://www.gnu.org/software/librejs/) banner to Anubis JavaScript to allow LibreJS users to run the challenge
- Added a wait with button continue + 30 second auto continue after 30s if you click "Why am I seeing this?"
- Fixed a typo in the challenge page title.
- Disabled running integration tests on Windows hosts due to it's reliance on posix features (see [#133](https://github.com/TecharoHQ/anubis/pull/133#issuecomment-2764732309)).
- Added support for passing the ed25519 signing key in a file with `-ed25519-private-key-hex-file` or `ED25519_PRIVATE_KEY_HEX_FILE`.

## v1.15.1

Zenos yae Galvus: Echo 1

Fixes a recurrence of [CVE-2025-24369](https://github.com/Xe/x/security/advisories/GHSA-56w8-8ppj-2p4f)
due to an incorrect logic change in a refactor. This allows an attacker to mint a valid
access token by passing any SHA-256 hash instead of one that matches the proof-of-work
test.

This case has been added as a regression test. It was not when CVE-2025-24369 was released
due to the project not having the maturity required to enable this kind of regression testing.

## v1.15.0

Zenos yae Galvus

> Yes...the coming days promise to be most interesting. Most interesting.

Headline changes:

- ed25519 signing keys for Anubis can be stored in the flag `--ed25519-private-key-hex` or envvar `ED25519_PRIVATE_KEY_HEX`; if one is not provided when Anubis starts, a new one is generated and logged
- Add the ability to set the cookie domain with the envvar `COOKIE_DOMAIN=techaro.lol` for all domains under `techaro.lol`
- Add the ability to set the cookie partitioned flag with the envvar `COOKIE_PARTITIONED=true`

Many other small changes were made, including but not limited to:

- Fixed and clarified installation instructions
- Introduced integration tests using Playwright
- Refactor & Split up Anubis into cmd and lib.go
- Fixed bot check to only apply if address range matches
- Fix default difficulty setting that was broken in a refactor
- Linting fixes
- Make dark mode diff lines readable in the documentation
- Fix CI based browser smoke test

Users running Anubis' test suite may run into issues with the integration tests on Windows hosts. This is a known issue and will be fixed at some point in the future. In the meantime, use the Windows Subsystem for Linux (WSL).

## v1.14.2

Livia sas Junius: Echo 2

- Remove default RSS reader rule as it may allow for a targeted attack against rails apps
  [#67](https://github.com/TecharoHQ/anubis/pull/67)
- Whitelist MojeekBot in botPolicies [#47](https://github.com/TecharoHQ/anubis/issues/47)
- botPolicies regex has been cleaned up [#66](https://github.com/TecharoHQ/anubis/pull/66)

## v1.14.1

Livia sas Junius: Echo 1

- Set the `X-Real-Ip` header based on the contents of `X-Forwarded-For`
  [#62](https://github.com/TecharoHQ/anubis/issues/62)

## v1.14.0

Livia sas Junius

> Fail to do as my lord commands...and I will spare him the trouble of blocking you.

- Add explanation of what Anubis is doing to the challenge page [#25](https://github.com/TecharoHQ/anubis/issues/25)
- Administrators can now define artificially hard challenges using the "slow" algorithm:

  ```json
  {
    "name": "generic-bot-catchall",
    "user_agent_regex": "(?i:bot|crawler)",
    "action": "CHALLENGE",
    "challenge": {
      "difficulty": 16,
      "report_as": 4,
      "algorithm": "slow"
    }
  }
  ```

  This allows administrators to cause particularly malicious clients to use unreasonable amounts of CPU. The UI will also lie to the client about the difficulty.

- Docker images now explicitly call `docker.io/library/<thing>` to increase compatibility with Podman et. al
  [#21](https://github.com/TecharoHQ/anubis/pull/21)
- Don't overflow the image when browser windows are small (eg. on phones)
  [#27](https://github.com/TecharoHQ/anubis/pull/27)
- Lower the default difficulty to 4 from 5
- Don't duplicate work across multiple threads [#36](https://github.com/TecharoHQ/anubis/pull/36)
- Documentation has been moved to https://anubis.techaro.lol/ with sources in docs/
- Removed several visible AI artifacts (e.g., 6 fingers) [#37](https://github.com/TecharoHQ/anubis/pull/37)
- [KagiBot](https://kagi.com/bot) is allowed through the filter [#44](https://github.com/TecharoHQ/anubis/pull/44)
- Fixed hang when navigator.hardwareConcurrency is undefined
- Support Unix domain sockets [#45](https://github.com/TecharoHQ/anubis/pull/45)
- Allow filtering by remote addresses:

  ```json
  {
    "name": "qwantbot",
    "user_agent_regex": "\\+https\\:\\/\\/help\\.qwant\\.com/bot/",
    "action": "ALLOW",
    "remote_addresses": ["91.242.162.0/24"]
  }
  ```

  This also works at an IP range level:

  ```json
  {
    "name": "internal-network",
    "action": "ALLOW",
    "remote_addresses": ["100.64.0.0/10"]
  }
  ```

## 1.13.0

- Proof-of-work challenges are drastically sped up [#19](https://github.com/TecharoHQ/anubis/pull/19)
- Docker images are now built with the timestamp set to the commit timestamp
- The README now points to TecharoHQ/anubis instead of Xe/x
- Images are built using ko instead of `docker buildx build`
  [#13](https://github.com/TecharoHQ/anubis/pull/13)

## 1.12.1

- Phrasing in the `<noscript>` warning was replaced from its original placeholder text to
  something more suitable for general consumption
  ([fd6903a](https://github.com/TecharoHQ/anubis/commit/fd6903aeed315b8fddee32890d7458a9271e4798)).
- Footer links on the check page now point to Techaro's brand
  ([4ebccb1](https://github.com/TecharoHQ/anubis/commit/4ebccb197ec20d024328d7f92cad39bbbe4d6359))
- Anubis was imported from [Xe/x](https://github.com/Xe/x).

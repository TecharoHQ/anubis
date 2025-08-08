---
sidebar_position: 999
---

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

<!-- This changes the project to: -->

- Downstream consumers can change the default [log/slog#Logger](https://pkg.go.dev/log/slog#Logger) instance that Anubis uses by setting `opts.Logger` to your slog instance of choice ([#864](https://github.com/TecharoHQ/anubis/issues/864)).
- The [Thoth client](https://anubis.techaro.lol/docs/admin/thoth) is now public in the repo instead of being an internal package.
- [Custom-AsyncHttpClient](https://github.com/AsyncHttpClient/async-http-client)'s default User-Agent has an increased weight by default ([#852](https://github.com/TecharoHQ/anubis/issues/852)).
- The [`segments`](./admin/configuration/expressions.mdx#segments) function was added for splitting a path into its slash-separated segments.
- When issuing a challenge, Anubis stores information about that challenge into the store. That stored information is later used to validate challenge responses. This works around nondeterminism in bot rules. ([#917](https://github.com/TecharoHQ/anubis/issues/917))
- When parsing [Open Graph tags](./admin/configuration/open-graph.mdx), add any URLs found in the responses to a temporary "allow cache" so that social preview images work.
- Proof of work solving has had a complete overhaul and rethink based on feedback from browser engine developers, frontend experts, and overall performance profiling.
- One of the biggest sources of lag in Firefox has been eliminated: the use of WebCrypto. Now whenever Anubis detects the client is using Firefox (or Pale Moon), it will swap over to a pure-JS implementation of SHA-256 for speed.
- Optimize the performance of the pure-JS Anubis solver.
- Web Workers are stored as dedicated JavaScript files in `static/js/workers/*.mjs`.
- Pave the way for non-SHA256 solver methods and eventually one that uses WebAssembly (or WebAssembly code compiled to JS for those that disable WebAssembly).
- Legacy JavaScript code has been eliminated.
- The contact email in the LibreJS header has been changed.
- The hard dependency on WebCrypto has been removed, allowing a proof of work challenge to work over plain (unencrypted) HTTP.
- Firefox for Android support has been fixed by embedding the challenge ID into the pass-challenge route. This also fixes some inconsistent issues with other mobile browsers.
- The Anubis version number is put in the footer of every page.
- The legacy JSON based policy file example has been removed and all documentation for how to write a policy file in JSON has been deleted. JSON based policy files will still work, but YAML is the superior option for Anubis configuration.
- A standard library HTTP server log message about HTTP pipelining not working has been filtered out of Anubis' logs. There is no action that can be taken about it.
- The `--cookie-prefix` flag has been fixed so that it is fully respected.

### Breaking changes

- The "slow" frontend solver has been removed in order to reduce maintenance burden. Any existing uses of it will still work, but issue a warning upon startup asking administrators to upgrade to the "fast" frontend solver.

## v1.21.3: Minfilia Warde - Echo 3

### Added

#### New locales

Anubis now supports these new languages:

- [Swedish](https://github.com/TecharoHQ/anubis/pull/913)

### Fixes

#### Fixes a problem with nonstandard URLs and redirects

Fixes [GHSA-jhjj-2g64-px7c](https://github.com/TecharoHQ/anubis/security/advisories/GHSA-jhjj-2g64-px7c).

This could allow an attacker to craft an Anubis pass-challenge URL that forces a redirect to nonstandard URLs, such as the `javascript:` scheme which executes arbitrary JavaScript code in a browser context when the user clicks the "Try again" button.

This has been fixed by disallowing any URLs without the scheme `http` or `https`.

Additionally, the "Try again" button has been fixed to completely ignore the user-supplied redirect location. It now redirects to the home page (`/`).

## v1.21.2: Minfilia Warde - Echo 2

This contained an incomplete fix for [GHSA-jhjj-2g64-px7c](https://github.com/TecharoHQ/anubis/security/advisories/GHSA-jhjj-2g64-px7c). Do not use this version.

## v1.21.1: Minfilia Warde - Echo 1

- Expired records are now properly removed from bbolt databases ([#848](https://github.com/TecharoHQ/anubis/pull/848)).
- Fix hanging on service restart ([#853](https://github.com/TecharoHQ/anubis/issues/853))

### Added

Anubis now supports the [`missingHeader`](./admin/configuration/expressions.mdx#missingHeader) to assert the absence of headers in requests.

#### New locales

Anubis now supports these new languages:

- [Czech](https://github.com/TecharoHQ/anubis/pull/849)
- [Finnish](https://github.com/TecharoHQ/anubis/pull/863)
- [Norwegian Bokmål](https://github.com/TecharoHQ/anubis/pull/855)
- [Norwegian Nynorsk](https://github.com/TecharoHQ/anubis/pull/855)
- [Russian](https://github.com/TecharoHQ/anubis/pull/882)

### Fixes

#### Fix ["error: can't get challenge"](https://github.com/TecharoHQ/anubis/issues/869) when details about a challenge can't be found in the server side state

v1.21.0 changed the core challenge flow to maintain information about challenges on the server side instead of only doing them via stateless idempotent generation functions and relying on details to not change. There was a subtle bug introduced in this change: if a client has an unknown challenge ID set in its test cookie, Anubis will clear that cookie and then throw an HTTP 500 error.

This has been fixed by making Anubis throw a new challenge page instead.

#### Fix event loop thrashing when solving a proof of work challenge

Previously the "fast" proof of work solver had a fragment of JavaScript that attempted to only post an update about proof of work progress to the main browser window every 1024 iterations. This fragment of JavaScript was subtly incorrect in a way that passed review but actually made the workers send an update back to the main thread every iteration. This caused a pileup of unhandled async calls (similar to a socket accept() backlog pileup in Unix) that caused stack space exhaustion.

This has been fixed in the following ways:

1. The complicated boolean logic has been totally removed in favour of a worker-local iteration counter.
2. The progress bar is updated by worker `0` instead of all workers.

Hopefully this should limit the event loop thrashing and let ia32 browsers (as well as any environment with a smaller stack size than amd64 and aarch64 seem to have) function normally when processing Anubis proof of work challenges.

#### Fix potential memory leak when discovering a solution

In some cases, the parallel solution finder in Anubis could cause all of the worker promises to leak due to the fact the promises were being improperly terminated. This was fixed by having Anubis debounce worker termination instead of allowing it to potentially recurse infinitely.

## v1.21.0: Minfilia Warde

> Please, be at ease. You are among friends here.

In this release, Anubis becomes internationalized, gains the ability to use system load as input to issuing challenges, finally fixes the "invalid response" after "success" bug, and more! Please read these notes before upgrading as the changes are big enough that administrators should take action to ensure that the upgrade goes smoothly.

### Big ticket changes

The biggest change is that the ["invalid response" after "success" bug](https://github.com/TecharoHQ/anubis/issues/564) is now finally fixed for good by totally rewriting how Anubis' challenge issuance flow works. Instead of generating challenge strings from request metadata (under the assumption that the values being compared against are stable), Anubis now generates random data for each challenge. This data is stored in the active [storage backend](./admin/policies.mdx#storage-backends) for up to 30 minutes. This also fixes [#746](https://github.com/TecharoHQ/anubis/issues/746) and other similar instances of this issue.

In order to reduce confusion, the "Success" interstitial that shows up when you pass a proof of work challenge has been removed.

#### Storage

Anubis now is able to store things persistently [in memory](./admin/policies.mdx#memory), [on the disk](./admin/policies.mdx#bbolt), or [in Valkey](./admin/policies.mdx#valkey) (this includes other compatible software). By default Anubis uses the in-memory backend. If you have an environment with mutable storage (even if it is temporary), be sure to configure the [`bbolt`](./admin/policies.mdx#bbolt) storage backend.

#### Localization

Anubis now supports localized responses. Locales can be added in [lib/localization/locales/](https://github.com/TecharoHQ/anubis/tree/main/lib/localization/locales). This release includes support for the following languages:

- [Brazilian Portugese](https://github.com/TecharoHQ/anubis/pull/726)
- [Chinese (Simplified)](https://github.com/TecharoHQ/anubis/pull/774)
- [Chinese (Traditional)](https://github.com/TecharoHQ/anubis/pull/759)
- English
- [Estonian](https://github.com/TecharoHQ/anubis/pull/783)
- [Filipino](https://github.com/TecharoHQ/anubis/pull/775)
- [French](https://github.com/TecharoHQ/anubis/pull/716)
- [German](https://github.com/TecharoHQ/anubis/pull/741)
- [Icelandic](https://github.com/TecharoHQ/anubis/pull/780)
- [Italian](https://github.com/TecharoHQ/anubis/pull/778)
- [Japanese](https://github.com/TecharoHQ/anubis/pull/772)
- [Spanish](https://github.com/TecharoHQ/anubis/pull/716)
- [Turkish](https://github.com/TecharoHQ/anubis/pull/751)

If facts or local regulations demand, you can set Anubis default language with the `FORCED_LANGUAGE` environment variable or the `--forced-language` command line argument:

```sh
FORCED_LANGUAGE=de
```

#### Load average

Anubis can dynamically take action [based on the system load average](./admin/configuration/expressions.mdx#using-the-system-load-average), allowing you to write rules like this:

```yaml
## System load based checks.
# If the system is under high load for the last minute, add weight.
- name: high-load-average
  action: WEIGH
  expression: load_1m >= 10.0 # make sure to end the load comparison in a .0
  weight:
    adjust: 20

# If it is not for the last 15 minutes, remove weight.
- name: low-load-average
  action: WEIGH
  expression: load_15m <= 4.0 # make sure to end the load comparison in a .0
  weight:
    adjust: -10
```

Something to keep in mind about system load average is that it is not aware of the number of cores the system has. If you have a 16 core system that has 16 processes running but none of them is hogging the CPU, then you will get a load average below 16. If you are in doubt, make your "high load" metric at least two times the number of CPU cores and your "low load" metric at least half of the number of CPU cores. For example:

|      Kind | Core count | Load threshold |
| --------: | :--------- | :------------- |
| high load | 4          | `8.0`          |
|  low load | 4          | `2.0`          |
| high load | 16         | `32.0`         |
|  low load | 16         | `8`            |

Also keep in mind that this does not account for other kinds of latency like I/O latency. A system can have its web applications unresponsive due to high latency from a MySQL server but still have that web application server report a load near or at zero.

### Other features and fixes

There are a bunch of other assorted features and fixes too:

- Add `COOKIE_SECURE` option to set the cookie [Secure flag](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/Cookies#block_access_to_your_cookies)
- Sets cookie defaults to use [SameSite: None](https://web.dev/articles/samesite-cookies-explained)
- Determine the `BIND_NETWORK`/`--bind-network` value from the bind address ([#677](https://github.com/TecharoHQ/anubis/issues/677)).
- Implement a [development container](https://containers.dev/) manifest to make contributions easier.
- Fix dynamic cookie domains functionality ([#731](https://github.com/TecharoHQ/anubis/pull/731))
- Add option for custom cookie prefix ([#732](https://github.com/TecharoHQ/anubis/pull/732))
- Make the [Open Graph](./admin/configuration/open-graph.mdx) subsystem and DNSBL subsystem use [storage backends](./admin/policies.mdx#storage-backends) instead of storing everything in memory by default.
- Allow [Common Crawl](https://commoncrawl.org/) by default so scrapers have less incentive to scrape
- The [bbolt storage backend](./admin/policies.mdx#bbolt) now runs its cleanup every hour instead of every five minutes.
- Don't block Anubis starting up if [Thoth](./admin/thoth.mdx) health checks fail.
- A race condition involving [opening two challenge pages at once in different tabs](https://github.com/TecharoHQ/anubis/issues/832) causing one of them to fail has been fixed.
- The "Try again" button on the error page has been fixed. Previously it meant "try the solution again" instead of "try the challenge again".
- In certain cases, a user could be stuck with a test cookie that is invalid, locking them out of the service for up to half an hour. This has been fixed with better validation of this case and clearing the cookie.
- Start exposing JA4H fingerprints for later use in CEL expressions.
- Add `/healthz` route for use in platform-based health checks.

### Potentially breaking changes

We try to introduce breaking changes as much as possible, but these are the changes that may be relevant for you as an administrator:

#### Challenge format change

Previously Anubis did no accounting for challenges that it issued. This means that if Anubis restarted during a client, the client would be able to proceed once Anubis came back online.

During the upgrade to v1.21.0 and when v1.21.0 (or later) restarts with the [in-memory storage backend](./admin/policies.mdx#memory), you may see a higher rate of failed challenges than normal. If this persists beyond a few minutes, [open an issue](https://github.com/TecharoHQ/anubis/issues/new).

If you are using the in-memory storage backend, please consider using [a different storage backend](./admin/policies.mdx#storage-backends).

#### Systemd service changes

The following potentially breaking change applies to native installs with systemd only:

Each instance of systemd service template now has a unique `RuntimeDirectory`, as opposed to each instance of the service sharing a `RuntimeDirectory`. This change was made to avoid [the `RuntimeDirectory` getting nuked any time one of the Anubis instances restarts](https://github.com/TecharoHQ/anubis/issues/748).

If you configured Anubis' unix sockets to listen on `/run/anubis/foo.sock` for instance `anubis@foo`, you will need to configure Anubis to listen on `/run/anubis/foo/foo.sock` and additionally configure your HTTP load balancer as appropriate.

If you need the legacy behaviour, install this [systemd unit dropin](https://www.flatcar.org/docs/latest/setup/systemd/drop-in-units/):

```systemd
# /etc/systemd/system/anubis@.service.d/50-runtimedir.conf
[Service]
RuntimeDirectory=anubis
```

Just keep in mind that this will cause problems when Anubis restarts.

## v1.20.0: Thancred Waters

The big ticket items are as follows:

- Implement a no-JS challenge method: [`metarefresh`](./admin/configuration/challenges/metarefresh.mdx) ([#95](https://github.com/TecharoHQ/anubis/issues/95))
- Implement request "weight", allowing administrators to customize the behaviour of Anubis based on specific criteria
- Implement GeoIP and ASN based checks via [Thoth](https://anubis.techaro.lol/docs/admin/thoth) ([#206](https://github.com/TecharoHQ/anubis/issues/206))
- Add [custom weight thresholds](./admin/configuration/thresholds.mdx) via CEL ([#688](https://github.com/TecharoHQ/anubis/pull/688))
- Move Open Graph configuration [to the policy file](./admin/configuration/open-graph.mdx)
- Enable support for Open Graph metadata to be returned by default instead of doing lookups against the target
- Add `robots2policy` CLI utility to convert robots.txt files to Anubis challenge policies using CEL expressions ([#409](https://github.com/TecharoHQ/anubis/issues/409))
- Refactor challenge presentation logic to use a challenge registry
- Allow challenge implementations to register HTTP routes
- [Imprint/Impressum support](./admin/configuration/impressum.mdx) ([#362](https://github.com/TecharoHQ/anubis/issues/362))
- Fix "invalid response" after "Success!" in Chromium ([#564](https://github.com/TecharoHQ/anubis/issues/564))

A lot of performance improvements have been made:

- Replace internal SHA256 hashing with xxhash for 4-6x performance improvement in policy evaluation and cache operations
- Optimized the OGTags subsystem with reduced allocations and runtime per request by up to 66%
- Replace cidranger with bart for IP range checking, improving IP matching performance by 3-20x with zero heap
  allocations

And some cleanups/refactors were added:

- Fix OpenGraph passthrough ([#717](https://github.com/TecharoHQ/anubis/issues/717))
- Remove the unused `/test-error` endpoint and update the testing endpoint `/make-challenge` to only be enabled in
  development
- Add `--xff-strip-private` flag/envvar to toggle skipping X-Forwarded-For private addresses or not
- Bump AI-robots.txt to version 1.37
- Make progress bar styling more compatible (UXP, etc)
- Add `--strip-base-prefix` flag/envvar to strip the base prefix from request paths when forwarding to target servers
- Fix an off-by-one in the default threshold config
- Add functionality for HS512 JWT algorithm
- Add support for dynamic cookie domains with the `--cookie-dynamic-domain`/`COOKIE_DYNAMIC_DOMAIN` flag/envvar

Request weight is one of the biggest ticket features in Anubis. This enables Anubis to be much closer to a Web Application Firewall and when combined with custom thresholds allows administrators to have Anubis take advanced reactions. For more information about request weight, see [the request weight section](./admin/policies.mdx#request-weight) of the policy file documentation.

TL;DR when you have one or more WEIGHT rules like this:

```yaml
bots:
  - name: gitea-session-token
    action: WEIGH
    expression:
      all:
        - '"Cookie" in headers'
        - headers["Cookie"].contains("i_love_gitea=")
    # Remove 5 weight points
    weight:
      adjust: -5
```

You can configure custom thresholds like this:

```yaml
thresholds:
  - name: minimal-suspicion # This client is likely fine, its soul is lighter than a feather
    expression: weight < 0 # a feather weighs zero units
    action: ALLOW # Allow the traffic through

  # For clients that had some weight reduced through custom rules, give them a
  # lightweight challenge.
  - name: mild-suspicion
    expression:
      all:
        - weight >= 0
        - weight < 10
    action: CHALLENGE
    challenge:
      # https://anubis.techaro.lol/docs/admin/configuration/challenges/metarefresh
      algorithm: metarefresh
      difficulty: 1
      report_as: 1

  # For clients that are browser-like but have either gained points from custom
  # rules or report as a standard browser.
  - name: moderate-suspicion
    expression:
      all:
        - weight >= 10
        - weight < 20
    action: CHALLENGE
    challenge:
      # https://anubis.techaro.lol/docs/admin/configuration/challenges/proof-of-work
      algorithm: fast
      difficulty: 2 # two leading zeros, very fast for most clients
      report_as: 2

  # For clients that are browser like and have gained many points from custom
  # rules
  - name: extreme-suspicion
    expression: weight >= 20
    action: CHALLENGE
    challenge:
      # https://anubis.techaro.lol/docs/admin/configuration/challenges/proof-of-work
      algorithm: fast
      difficulty: 4
      report_as: 4
```

These thresholds apply when no other `ALLOW`, `DENY`, or `CHALLENGE` rule matches the request. `WEIGHT` rules add and remove request weight as needed:

```yaml
bots:
  - name: gitea-session-token
    action: WEIGH
    expression:
      all:
        - '"Cookie" in headers'
        - headers["Cookie"].contains("i_love_gitea=")
    # Remove 5 weight points
    weight:
      adjust: -5

  - name: bot-like-user-agent
    action: WEIGH
    expression: '"Bot" in userAgent'
    # Add 5 weight points
    weight:
      adjust: 5
```

Of note: the default "generic browser" rule assigns 10 weight points:

```yaml
# Generic catchall rule
- name: generic-browser
  user_agent_regex: >-
    Mozilla|Opera
  action: WEIGH
  weight:
    adjust: 10
```

Adjust this as you see fit.

## v1.19.1: Jenomis cen Lexentale - Echo 1

- Return `data/bots/ai-robots-txt.yaml` to avoid breaking configs [#599](https://github.com/TecharoHQ/anubis/issues/599)

## v1.19.0: Jenomis cen Lexentale

Mostly a bunch of small features, no big ticket things this time.

- Record if challenges were issued via the API or via embedded JSON in the challenge page HTML ([#531](https://github.com/TecharoHQ/anubis/issues/531))
- Ensure that clients that are shown a challenge support storing cookies
- Imprint the version number into challenge pages
- Encode challenge pages with gzip level 1
- Add PowerPC 64 bit little-endian builds (`GOARCH=ppc64le`)
- Add `check-spelling` for spell checking
- Add `--target-insecure-skip-verify` flag/envvar to allow Anubis to hit a self-signed HTTPS backend
- Minor adjustments to FreeBSD rc.d script to allow for more flexible configuration.
- Added Podman and Docker support for running Playwright tests
- Add a default rule to throw challenges when a request with the `X-Firefox-Ai` header is set
- Updated the nonce value in the challenge JWT cookie to be a string instead of a number
- Rename cookies in response to user feedback
- Ensure cookie renaming is consistent across configuration options
- Add Bookstack app in data
- Truncate everything but the first five characters of Accept-Language headers when making challenges
- Ensure client JavaScript is served with Content-Type text/javascript.
- Add `--target-host` flag/envvar to allow changing the value of the Host header in requests forwarded to the target service
- Bump AI-robots.txt to version 1.31
- Add `RuntimeDirectory` to systemd unit settings so native packages can listen over unix sockets
- Added SearXNG instance tracker whitelist policy
- Added Qualys SSL Labs whitelist policy
- Fixed cookie deletion logic ([#520](https://github.com/TecharoHQ/anubis/issues/520), [#522](https://github.com/TecharoHQ/anubis/pull/522))
- Add `--target-sni` flag/envvar to allow changing the value of the TLS handshake hostname in requests forwarded to the target service
- Fixed CEL expression matching validator to now properly error out when it receives empty expressions
- Added OpenRC init.d script
- Added `--version` flag
- Added `anubis_proxied_requests_total` metric to count proxied requests
- Add `Applebot` as "good" web crawler
- Reorganize AI/LLM crawler blocking into three separate stances, maintaining existing status quo as default
- Split out AI/LLM user agent blocking policies, adding documentation for each

## v1.18.0: Varis zos Galvus

The big ticket feature in this release is [CEL expression matching support](https://anubis.techaro.lol/docs/admin/configuration/expressions). This allows you to tailor your approach for the individual services you are protecting.

These can be as simple as:

```yaml
- name: allow-api-requests
  action: ALLOW
  expression:
    all:
      - '"Accept" in headers'
      - 'headers["Accept"] == "application/json"'
      - 'path.startsWith("/api/")'
```

Or as complicated as:

```yaml
- name: allow-git-clients
  action: ALLOW
  expression:
    all:
      - >-
        (
          userAgent.startsWith("git/") ||
          userAgent.contains("libgit") ||
          userAgent.startsWith("go-git") ||
          userAgent.startsWith("JGit/") ||
          userAgent.startsWith("JGit-")
        )
      - '"Git-Protocol" in headers'
      - headers["Git-Protocol"] == "version=2"
```

The docs have more information, but here's a tl;dr of the variables you have access to in expressions:

| Name            | Type                  | Explanation                                                                                                                               | Example                                                      |
| :-------------- | :-------------------- | :---------------------------------------------------------------------------------------------------------------------------------------- | :----------------------------------------------------------- |
| `headers`       | `map[string, string]` | The [headers](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers) of the request being processed.                        | `{"User-Agent": "Mozilla/5.0 Gecko/20100101 Firefox/137.0"}` |
| `host`          | `string`              | The [HTTP hostname](https://web.dev/articles/url-parts#host) the request is targeted to.                                                  | `anubis.techaro.lol`                                         |
| `method`        | `string`              | The [HTTP method](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Methods) in the request being processed.                    | `GET`, `POST`, `DELETE`, etc.                                |
| `path`          | `string`              | The [path](https://web.dev/articles/url-parts#pathname) of the request being processed.                                                   | `/`, `/api/memes/create`                                     |
| `query`         | `map[string, string]` | The [query parameters](https://web.dev/articles/url-parts#query) of the request being processed.                                          | `?foo=bar` -> `{"foo": "bar"}`                               |
| `remoteAddress` | `string`              | The IP address of the client.                                                                                                             | `1.1.1.1`                                                    |
| `userAgent`     | `string`              | The [`User-Agent`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/User-Agent) string in the request being processed. | `Mozilla/5.0 Gecko/20100101 Firefox/137.0`                   |

This will be made more elaborate in the future. Give me time. This is a [simple, lovable, and complete](https://longform.asmartbear.com/slc/) implementation of this feature so that administrators can get hacking ASAP.

Other changes:

- Use CSS variables to deduplicate styles
- Fixed native packages not containing the stdlib and botPolicies.yaml
- Change import syntax to allow multi-level imports
- Changed the startup logging to use JSON formatting as all the other logs do
- Added the ability to do [expression matching with CEL](./admin/configuration/expressions.mdx)
- Add a warning for clients that don't store cookies
- Disable Open Graph passthrough by default ([#435](https://github.com/TecharoHQ/anubis/issues/435))
- Clarify the license of the mascot images ([#442](https://github.com/TecharoHQ/anubis/issues/442))
- Started Suppressing 'Context canceled' errors from http in the logs ([#446](https://github.com/TecharoHQ/anubis/issues/446))

## v1.17.1: Asahi sas Brutus: Echo 1

- Added customization of authorization cookie expiration time with `--cookie-expiration-time` flag or envvar
- Updated the `OG_PASSTHROUGH` to be true by default, thereby allowing Open Graph tags to be passed through by default
- Added the ability to [customize Anubis' HTTP status codes](./admin/configuration/custom-status-codes.mdx) ([#355](https://github.com/TecharoHQ/anubis/issues/355))

## v1.17.0: Asahi sas Brutus

- Ensure regexes can't end in newlines ([#372](https://github.com/TecharoHQ/anubis/issues/372))
- Add documentation for default allow behavior (implicit rule)
- Enable [importing configuration snippets](./admin/configuration/import.mdx) ([#321](https://github.com/TecharoHQ/anubis/pull/321))
- Refactor check logic to be more generic and work on a Checker type
- Add more AI user agents based on the [ai.robots.txt](https://github.com/ai-robots-txt/ai.robots.txt) project
- Embedded challenge data in initial HTML response to improve performance
- Added support to use Nginx' `auth_request` directive with Anubis
- Added support to allow to restrict the allowed redirect domains
- Whitelisted [DuckDuckBot](https://duckduckgo.com/duckduckgo-help-pages/results/duckduckbot/) in botPolicies
- Improvements to build scripts to make them less independent of the build host
- Improved the Open Graph error logging
- Added `Opera` to the `generic-browser` bot policy rule
- Added FreeBSD rc.d script so can be run as a FreeBSD daemon
- Allow requests from the Internet Archive
- Added example nginx configuration to documentation
- Added example Apache configuration to the documentation [#277](https://github.com/TecharoHQ/anubis/issues/277)
- Move per-environment configuration details into their own pages
- Added support for running anubis behind a prefix (e.g. `/myapp`)
- Added headers support to bot policy rules
- Moved configuration file from JSON to YAML by default
- Added documentation on how to use Anubis with Traefik in Docker
- Improved error handling in some edge cases
- Disable `generic-bot-catchall` rule because of its high false positive rate in real-world scenarios
- Moved all CSS inline to the Xess package, changed colors to be CSS variables
- Set or append to `X-Forwarded-For` header unless the remote connects over a loopback address [#328](https://github.com/TecharoHQ/anubis/issues/328)
- Fixed mojeekbot user agent regex
- Reduce Anubis' paranoia with user cookies ([#365](https://github.com/TecharoHQ/anubis/pull/365))
- Added support for Open Graph passthrough while using unix sockets
- The Open Graph subsystem now passes the HTTP `HOST` header through to the origin
- Updated the `OG_PASSTHROUGH` to be true by default, thereby allowing Open Graph tags to be passed through by default

## v1.16.0

Fordola rem Lupis

> I want to make them pay! All of them! Everyone who ever mocked or looked down on me -- I want the power to make them pay!

The following features are the "big ticket" items:

- Added support for native Debian, Red Hat, and tarball packaging strategies including installation and use directions
- A prebaked tarball has been added, allowing distros to build Anubis like they could in v1.15.x
- The placeholder Anubis mascot has been replaced with a design by [CELPHASE](https://bsky.app/profile/celphase.bsky.social)
- Verification page now shows hash rate and a progress bar for completion probability
- Added support for [Open Graph tags](https://ogp.me/) when rendering the challenge page. This allows for social previews to be generated when sharing the challenge page on social media platforms ([#195](https://github.com/TecharoHQ/anubis/pull/195))
- Added support for passing the ed25519 signing key in a file with `-ed25519-private-key-hex-file` or `ED25519_PRIVATE_KEY_HEX_FILE`

The other small fixes have been made:

- Added a periodic cleanup routine for the decaymap that removes expired entries, ensuring stale data is properly pruned
- Added a no-store Cache-Control header to the challenge page
- Hide the directory listings for Anubis' internal static content
- Changed `--debug-x-real-ip-default` to `--use-remote-address`, getting the IP address from the request's socket address instead
- DroneBL lookups have been disabled by default
- Static asset builds are now done on demand instead of the results being committed to source control
- The Dockerfile has been removed as it is no longer in use
- Developer documentation has been added to the docs site
- Show more errors when some predictable challenge page errors happen ([#150](https://github.com/TecharoHQ/anubis/issues/150))
- Added the `--debug-benchmark-js` flag for testing proof-of-work performance during development
- Use `TrimSuffix` instead of `TrimRight` on containerbuild
- Fix the startup logs to correctly show the address and port the server is listening on
- Add [LibreJS](https://www.gnu.org/software/librejs/) banner to Anubis JavaScript to allow LibreJS users to run the challenge
- Added a wait with button continue + 30 second auto continue after 30s if you click "Why am I seeing this?"
- Fixed a typo in the challenge page title
- Disabled running integration tests on Windows hosts due to it's reliance on posix features (see [#133](https://github.com/TecharoHQ/anubis/pull/133#issuecomment-2764732309))
- Fixed minor typos
- Added a Makefile to enable comfortable workflows for downstream packagers
- Added `zizmor` for GitHub Actions static analysis
- Fixed most `zizmor` findings
- Enabled Dependabot
- Added an air config for autoreload support in development ([#195](https://github.com/TecharoHQ/anubis/pull/195))
- Added an `--extract-resources` flag to extract static resources to a local folder
- Add noindex flag to all Anubis pages ([#227](https://github.com/TecharoHQ/anubis/issues/227))
- Added `WEBMASTER_EMAIL` variable, if it is present then display that email address on error pages ([#235](https://github.com/TecharoHQ/anubis/pull/235), [#115](https://github.com/TecharoHQ/anubis/issues/115))
- Hash pinned all GitHub Actions

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
- Lower the default difficulty to 5 from 4
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
- Anubis was imported from [Xe/x](https://github.com/Xe/x)

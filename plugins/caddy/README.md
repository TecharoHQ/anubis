# Anubis Caddy Plugin

A native [Caddy](https://caddyserver.com) middleware module for
[Anubis](https://github.com/TecharoHQ/anubis).  
Both are Go programs, so Anubis can be compiled directly into Caddy with no
separate process, no forward-auth round-trips, and no extra network hops.

---

## Installation

### With xcaddy (recommended)

```sh
xcaddy build \
  --with github.com/TecharoHQ/anubis/plugins/caddy
```

### As a Go import (custom Caddy fork)

Add the blank import to your `main.go`:

```go
import _ "github.com/TecharoHQ/anubis/plugins/caddy"
```

---

## Quick start

### Caddyfile

```caddyfile
{
    order anubis before reverse_proxy
}

example.com {
    anubis {
        policy_file   /etc/anubis/policy.yaml   # optional; built-in policy used when omitted
        difficulty    5
        public_url    https://example.com
        cookie_domain example.com
        redirect_domains example.com *.example.com
        serve_robots_txt
    }

    reverse_proxy localhost:3000
}
```

Anubis inspects every request before it reaches the `reverse_proxy`
directive. Requests that pass the challenge (or are explicitly allowed by
policy) are forwarded normally. Bots get the proof-of-work challenge page.

### JSON config

```json
{
  "handler": "anubis",
  "policy_file": "/etc/anubis/policy.yaml",
  "difficulty": 5,
  "public_url": "https://example.com",
  "cookie_domain": "example.com",
  "redirect_domains": ["example.com", "*.example.com"],
  "serve_robots_txt": true
}
```

---

## Caddyfile directives reference

| Subdirective | Type | Default | Description |
|---|---|---|---|
| `policy_file <path>` | string | built-in | Path to Anubis bot-policy YAML. |
| `difficulty <n>` | int | 4 | Proof-of-work difficulty (leading zeroes). |
| `cookie_domain <domain>` | string | | Pin cookie Domain to a value. |
| `cookie_dynamic_domain` | flag | false | Derive cookie domain from request Host. |
| `cookie_expiration <dur>` | duration | 168h | How long cookies stay valid. |
| `cookie_insecure` | flag | false | Remove the `Secure` flag from cookies. |
| `cookie_same_site <mode>` | string | None | `None`, `Lax`, `Strict`, or `Default`. |
| `cookie_partitioned` | flag | false | Enable CHIPS partitioned cookies. |
| `base_prefix <prefix>` | string | | URL prefix Anubis is mounted under. |
| `strip_base_prefix` | flag | false | Strip base prefix before forwarding. |
| `redirect_domains <d>…` | strings | any | Allowlist of post-challenge redirect targets. `*` globs supported. |
| `webmaster_email <email>` | string | | Email shown on deny page for appeals. |
| `serve_robots_txt` | flag | false | Serve a `robots.txt` that disallows all bots. |
| `public_url <url>` | string | | Externally reachable Anubis URL (used for forward-auth redirects). |
| `hs512_secret <secret>` | string | | Sign JWTs with HS512 instead of ED25519. |
| `ed25519_private_key_hex <hex>` | string | random | Persistent ED25519 signing key (32-byte seed, hex-encoded). |
| `jwt_restriction_header <h>` | string | X-Real-IP | Bind JWTs to the value of this request header. |
| `difficulty_in_jwt` | flag | false | Embed challenge difficulty in JWT claims. |
| `use_remote_addr` | flag | false | Auto-set `X-Real-Ip` from the TCP remote address. Only enable when Caddy is directly internet-facing. |

---

## Persistent signing key

By default Anubis generates a fresh ED25519 key each time the process starts.
Any existing cookies signed with the old key will be rejected, forcing visitors
to solve the challenge again.  To avoid this, generate a persistent key once
and store it in your Caddyfile or a secret manager:

```sh
# Generate a 32-byte seed and print it as hex
openssl rand -hex 32
```

Then set `ed25519_private_key_hex` to that value, or use `hs512_secret` if you
prefer HMAC-SHA512 signatures.

---

## X-Real-Ip header

Anubis requires the `X-Real-Ip` header to be present on every request so it
can apply IP-based policy rules. When Caddy sits behind another reverse proxy
(e.g. Cloudflare, nginx, another Caddy instance) that proxy should set
`X-Real-Ip` for you.

When Caddy is directly internet-facing, enable `use_remote_addr` in the Anubis
block:

```caddyfile
anubis {
    use_remote_addr
}
```

This sets `X-Real-Ip` from the TCP remote address for every request that
doesn't already have the header.

---

## Multi-site setup

A single Caddy instance can protect multiple sites with independent Anubis
configurations:

```caddyfile
{
    order anubis before reverse_proxy
}

api.example.com {
    anubis {
        difficulty 8          # stricter for the API
        cookie_domain api.example.com
    }
    reverse_proxy localhost:4000
}

www.example.com {
    anubis {
        policy_file /etc/anubis/relaxed.yaml
        cookie_domain example.com
        cookie_dynamic_domain
    }
    reverse_proxy localhost:3000
}
```

Each site has its own independent Anubis server instance with its own policy,
key material, and cookie settings.

---

## Forward-auth compatibility

If you run Anubis as a sidecar and use Caddy's `forward_auth` directive today,
this module is a drop-in improvement: delete the `forward_auth` block and
replace it with an `anubis` block. The module speaks the same cookie/JWT
protocol so previously issued cookies remain valid.

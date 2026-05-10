# Anubis Caddy Module

This package registers Anubis as a native Caddy HTTP middleware handler. It can
be built directly into Caddy with `xcaddy`:

```sh
xcaddy build --with github.com/TecharoHQ/anubis/plugins/caddy
```

The module registers the `anubis` Caddyfile directive as
`http.handlers.anubis` and orders it after Caddy's `templates` directive, before
response-producing handlers such as `respond`, `reverse_proxy`, and
`file_server`.

## Caddyfile

```caddyfile
example.com {
	anubis {
		policy_file /etc/anubis/botPolicies.yaml
		difficulty 5
		use_remote_addr
		redirect_domains example.com *.example.com
	}

	reverse_proxy localhost:3000
}
```

When `policy_file` is omitted, Anubis uses its built-in policy. If Caddy is
directly internet-facing, enable `use_remote_addr` so Anubis can populate
`X-Real-Ip` from the client connection. If another trusted proxy sits in front
of Caddy, configure that proxy or Caddy to set `X-Real-Ip` instead.

## Configuration scope

Each `anubis` directive provisions its own Anubis policy, cookie settings, and
next-handler bridge, so Caddy matchers and routes can control where the
middleware runs. Anubis currently keeps `base_prefix`, `public_url`,
`forced_language`, and `use_simplified_explanation` in package-level state, so
keep those settings consistent across `anubis` handlers in the same Caddy
process.

## Directives

| Subdirective | Description |
| --- | --- |
| `policy_file <path>` | Path to an Anubis bot policy YAML file. |
| `difficulty <n>` | Default proof-of-work difficulty. Defaults to Anubis' built-in default. |
| `log_level <level>` | Log level used while loading the policy. |
| `cookie_domain <domain>` | Pin the Anubis cookie domain. |
| `cookie_dynamic_domain` | Derive the cookie domain from the request host. |
| `cookie_expiration <duration>` | Authorization cookie lifetime. Defaults to Anubis' built-in default. |
| `cookie_secure <bool>` | Set or clear the Secure cookie flag. Defaults to true. |
| `cookie_insecure` | Shortcut for `cookie_secure false`. |
| `cookie_same_site <mode>` | `None`, `Lax`, `Strict`, or `Default`. Defaults to `None`. |
| `cookie_partitioned` | Enable partitioned cookies. |
| `base_prefix <prefix>` | Serve Anubis endpoints below a path prefix. |
| `strip_base_prefix` | Strip `base_prefix` before forwarding allowed requests. |
| `redirect_domains <domain>...` | Allowlist redirect destinations after a challenge. |
| `webmaster_email <email>` | Contact address shown on challenge or deny pages. |
| `serve_robots_txt` | Serve Anubis' built-in robots.txt. |
| `public_url <url>` | Public URL used when constructing challenge redirects. |
| `hs512_secret <secret>` | Sign JWTs with HS512. Mutually exclusive with ED25519 keys. |
| `ed25519_private_key_hex <hex>` | Hex-encoded 32-byte ED25519 seed for stable JWT signing. |
| `jwt_restriction_header <header>` | Bind JWTs to a request header. Defaults to `X-Real-IP`. |
| `difficulty_in_jwt` | Include challenge difficulty in JWT claims. |
| `use_remote_addr` | Populate `X-Real-Ip` from the TCP remote address when absent. |
| `forced_language <language>` | Force Anubis UI localization. |
| `use_simplified_explanation` | Use the simplified challenge explanation. |

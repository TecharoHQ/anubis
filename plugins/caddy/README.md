# Anubis Caddy Module

This module lets Caddy run Anubis as HTTP middleware.

Build a Caddy binary with:

```sh
xcaddy build --with github.com/TecharoHQ/anubis/plugins/caddy
```

Then use it in a route:

```Caddyfile
example.com {
	route {
		anubis {
			policy_file /etc/anubis/botPolicies.yaml
			difficulty 4
		}

		reverse_proxy localhost:3000
	}
}
```

When `policy_file` is omitted, Anubis uses its embedded default policy.

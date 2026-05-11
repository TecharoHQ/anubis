package caddy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	libanubis "github.com/TecharoHQ/anubis/lib"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(Anubis{})
	httpcaddyfile.RegisterHandlerDirective("anubis", parseCaddyfile)
}

// Anubis is a Caddy HTTP middleware that protects the next handler with Anubis.
type Anubis struct {
	PolicyFile string `json:"policy_file,omitempty"`
	Difficulty int    `json:"difficulty,omitempty"`
	BasePrefix string `json:"base_prefix,omitempty"`

	server *libanubis.Server
}

func (Anubis) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.anubis",
		New: func() caddy.Module { return new(Anubis) },
	}
}

func (m *Anubis) Provision(_ caddy.Context) error {
	difficulty := m.Difficulty
	if difficulty == 0 {
		difficulty = 4
	}

	policy, err := libanubis.LoadPoliciesOrDefault(context.Background(), m.PolicyFile, difficulty, "info", false)
	if err != nil {
		return fmt.Errorf("caddy: load anubis policy: %w", err)
	}

	server, err := libanubis.New(libanubis.Options{
		BasePrefix: m.BasePrefix,
		Policy:     policy,
		Logger:     slog.Default().With("subsystem", "anubis-caddy"),
	})
	if err != nil {
		return fmt.Errorf("caddy: create anubis server: %w", err)
	}

	m.server = server
	return nil
}

func (m Anubis) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	var nextErr error
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextErr = next.ServeHTTP(w, r)
	})

	m.server.HandlerWithNext(nextHandler).ServeHTTP(w, r)
	return nextErr
}

func (m *Anubis) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		for nesting := d.Nesting(); d.NextBlock(nesting); {
			switch d.Val() {
			case "policy_file":
				if !d.Args(&m.PolicyFile) {
					return d.ArgErr()
				}
			case "difficulty":
				if !d.NextArg() {
					return d.ArgErr()
				}
				var difficulty int
				if _, err := fmt.Sscanf(d.Val(), "%d", &difficulty); err != nil {
					return d.Errf("difficulty must be an integer: %v", err)
				}
				m.Difficulty = difficulty
			case "base_prefix":
				if !d.Args(&m.BasePrefix) {
					return d.ArgErr()
				}
			default:
				return d.Errf("unrecognized anubis option %q", d.Val())
			}
		}
	}
	return nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m Anubis
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return &m, err
}

var (
	_ caddy.Provisioner           = (*Anubis)(nil)
	_ caddyfile.Unmarshaler       = (*Anubis)(nil)
	_ caddyhttp.MiddlewareHandler = (*Anubis)(nil)
)

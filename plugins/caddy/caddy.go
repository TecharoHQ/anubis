// Package caddy provides an Anubis middleware plugin for the Caddy web server.
//
// Build with xcaddy:
//
//	xcaddy build --with github.com/TecharoHQ/anubis/plugins/caddy
//
// Or add to your Caddy fork's main.go imports:
//
//	import _ "github.com/TecharoHQ/anubis/plugins/caddy"
package caddy

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"

	libanubis "github.com/TecharoHQ/anubis/lib"

	// Register all built-in store backends.
	_ "github.com/TecharoHQ/anubis/lib/store/all"
)

func init() {
	caddy.RegisterModule(Middleware{})
	httpcaddyfile.RegisterHandlerDirective("anubis", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("anubis", httpcaddyfile.Before, "reverse_proxy")
}

// contextKey is an unexported type used to store the Caddy next-handler in
// the request context so that the Anubis lib.Server can call it without
// needing to capture it at construction time.
type contextKey struct{}

// contextualNext is an http.Handler that retrieves the per-request Caddy
// next handler from the request context and delegates to it. An instance of
// this type is passed to lib.New as the Options.Next value, allowing a single
// lib.Server to serve all requests while still forwarding to the correct Caddy
// handler chain for each one.
type contextualNext struct{}

func (contextualNext) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if next, ok := r.Context().Value(contextKey{}).(caddyhttp.Handler); ok {
		if err := next.ServeHTTP(w, r); err != nil {
			// Caddy errors can't propagate through net/http; log them via the
			// response writer if possible and fall back to a 500.
			if w.Header().Get("Content-Type") == "" {
				http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
			}
		}
	}
}

// Middleware is the Caddy HTTP middleware module for Anubis.
//
// It wraps every request with the Anubis bot-challenge logic before forwarding
// to the next handler in the Caddy chain, with no need for a separate process
// or forward-auth round-trip.
type Middleware struct {
	// PolicyFile is the path to an Anubis bot-policy YAML file. When empty,
	// the built-in default policy is used.
	PolicyFile string `json:"policy_file,omitempty"`

	// Difficulty is the default proof-of-work difficulty (number of leading
	// zeroes). Defaults to 4 when unset.
	Difficulty int `json:"difficulty,omitempty"`

	// CookieDomain pins the cookie Domain attribute to a specific value.
	CookieDomain string `json:"cookie_domain,omitempty"`

	// CookieDynamicDomain, when true, derives the cookie domain automatically
	// from the effective TLD+1 of each request's Host header.
	CookieDynamicDomain bool `json:"cookie_dynamic_domain,omitempty"`

	// CookieExpiration is how long issued cookies remain valid. Defaults to 7
	// days when unset.
	CookieExpiration caddy.Duration `json:"cookie_expiration,omitempty"`

	// CookieSecure sets the Secure flag on cookies. Defaults to true.
	CookieSecure *bool `json:"cookie_secure,omitempty"`

	// CookieSameSite sets the SameSite attribute. Valid values: None, Lax,
	// Strict, Default. Defaults to "None".
	CookieSameSite string `json:"cookie_same_site,omitempty"`

	// CookiePartitioned enables CHIPS (Partitioned cookies) when true.
	CookiePartitioned bool `json:"cookie_partitioned,omitempty"`

	// BasePrefix is the URL path prefix under which Anubis is mounted, e.g.
	// "/myapp". Leave empty when serving from the root.
	BasePrefix string `json:"base_prefix,omitempty"`

	// StripBasePrefix, when true, removes BasePrefix before forwarding
	// requests upstream.
	StripBasePrefix bool `json:"strip_base_prefix,omitempty"`

	// RedirectDomains is an allowlist of domains Anubis may redirect to after
	// a successful challenge. Supports '*' glob wildcards. When empty, all
	// domains are permitted.
	RedirectDomains []string `json:"redirect_domains,omitempty"`

	// WebmasterEmail, when set, is shown on the deny page so that users can
	// appeal.
	WebmasterEmail string `json:"webmaster_email,omitempty"`

	// ServeRobotsTXT, when true, serves Anubis's built-in robots.txt that
	// disallows all crawlers.
	ServeRobotsTXT bool `json:"serve_robots_txt,omitempty"`

	// PublicURL is the externally reachable base URL of this Anubis instance,
	// used when constructing redirect URLs for forward-auth setups.
	PublicURL string `json:"public_url,omitempty"`

	// HS512Secret, when set, is used to sign JWTs instead of an ED25519 key.
	HS512Secret string `json:"hs512_secret,omitempty"`

	// ED25519PrivateKeyHex is an optional hex-encoded ED25519 private key seed
	// (32 bytes). When empty, a random key is generated per process start.
	ED25519PrivateKeyHex string `json:"ed25519_private_key_hex,omitempty"`

	// JWTRestrictionHeader binds issued JWTs to the value of a specific
	// request header (e.g. "X-Real-IP") so they cannot be replayed by another
	// client. Defaults to "X-Real-IP".
	JWTRestrictionHeader string `json:"jwt_restriction_header,omitempty"`

	// DifficultyInJWT, when true, embeds the challenge difficulty in the JWT
	// claims.
	DifficultyInJWT bool `json:"difficulty_in_jwt,omitempty"`

	// UseRemoteAddr, when true, sets the X-Real-Ip header from the network
	// remote address for each request. Enable this only when Anubis sits
	// directly on the internet; behind another proxy you should let that proxy
	// set the header instead.
	UseRemoteAddr bool `json:"use_remote_addr,omitempty"`

	// server is the provisioned Anubis lib.Server instance.
	server *libanubis.Server
}

// CaddyModule implements caddy.Module.
func (Middleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.anubis",
		New: func() caddy.Module { return new(Middleware) },
	}
}

// Provision implements caddy.Provisioner. It loads the bot policy and
// constructs the Anubis Server that will handle requests.
func (m *Middleware) Provision(ctx caddy.Context) error {
	// ctx.Logger() returns *zap.Logger; Anubis uses log/slog. We discard the
	// Caddy logger here and let Anubis construct its own default slog logger.
	_ = ctx.Logger()

	difficulty := m.Difficulty
	if difficulty == 0 {
		difficulty = 4 // anubis.DefaultDifficulty
	}

	parsedPolicy, err := libanubis.LoadPoliciesOrDefault(
		context.Background(),
		m.PolicyFile,
		difficulty,
		"INFO",
	)
	if err != nil {
		return fmt.Errorf("anubis caddy: failed to load policy: %w", err)
	}

	opts := libanubis.Options{
		Next:                 contextualNext{},
		Policy:               parsedPolicy,
		CookieDomain:         m.CookieDomain,
		CookieDynamicDomain:  m.CookieDynamicDomain,
		CookieExpiration:     time.Duration(m.CookieExpiration),
		CookiePartitioned:    m.CookiePartitioned,
		BasePrefix:           m.BasePrefix,
		StripBasePrefix:      m.StripBasePrefix,
		RedirectDomains:      m.RedirectDomains,
		WebmasterEmail:       m.WebmasterEmail,
		ServeRobotsTXT:       m.ServeRobotsTXT,
		PublicUrl:            m.PublicURL,
		JWTRestrictionHeader: m.JWTRestrictionHeader,
		DifficultyInJWT:      m.DifficultyInJWT,
	}

	// Cookie expiration defaults.
	if opts.CookieExpiration == 0 {
		opts.CookieExpiration = 7 * 24 * time.Hour
	}

	// Cookie Secure defaults to true.
	opts.CookieSecure = true
	if m.CookieSecure != nil {
		opts.CookieSecure = *m.CookieSecure
	}

	// SameSite handling.
	opts.CookieSameSite = parseSameSite(m.CookieSameSite)

	// JWT restriction header default.
	if opts.JWTRestrictionHeader == "" {
		opts.JWTRestrictionHeader = "X-Real-IP"
	}

	// ED25519 key.
	if m.ED25519PrivateKeyHex != "" {
		priv, err := keyFromHex(m.ED25519PrivateKeyHex)
		if err != nil {
			return fmt.Errorf("anubis caddy: invalid ed25519_private_key_hex: %w", err)
		}
		opts.ED25519PrivateKey = priv
	}

	// HS512 secret.
	if m.HS512Secret != "" {
		opts.HS512Secret = []byte(m.HS512Secret)
	}

	srv, err := libanubis.New(opts)
	if err != nil {
		return fmt.Errorf("anubis caddy: failed to create server: %w", err)
	}
	m.server = srv

	return nil
}

// Validate implements caddy.Validator.
func (m *Middleware) Validate() error {
	if m.server == nil {
		return fmt.Errorf("anubis caddy: server was not provisioned")
	}
	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler. It injects the Caddy next
// handler into the request context so that the Anubis lib.Server can reach it
// via contextualNext, then delegates to the Anubis request handler.
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if m.UseRemoteAddr && r.Header.Get("X-Real-Ip") == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		// Clone the request so we don't mutate the original headers.
		r = r.Clone(r.Context())
		r.Header.Set("X-Real-Ip", host)
	}

	ctx := context.WithValue(r.Context(), contextKey{}, next)
	r = r.WithContext(ctx)

	m.server.ServeHTTP(w, r)
	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
//
// Supported Caddyfile syntax:
//
//	anubis [<policy_file>] {
//	    policy_file    <path>
//	    difficulty     <int>
//	    cookie_domain  <domain>
//	    cookie_dynamic_domain
//	    cookie_expiration <duration>
//	    cookie_insecure
//	    cookie_same_site  <None|Lax|Strict|Default>
//	    cookie_partitioned
//	    base_prefix       <prefix>
//	    strip_base_prefix
//	    redirect_domains  <domain> [<domain> ...]
//	    webmaster_email   <email>
//	    serve_robots_txt
//	    public_url        <url>
//	    hs512_secret      <secret>
//	    ed25519_private_key_hex <hex>
//	    jwt_restriction_header  <header>
//	    difficulty_in_jwt
//	    use_remote_addr
//	}
func (m *Middleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume directive name

	// Optional inline policy file argument: `anubis /path/to/policy.yaml`
	if d.NextArg() {
		m.PolicyFile = d.Val()
	}

	for d.NextBlock(0) {
		switch d.Val() {
		case "policy_file":
			if !d.NextArg() {
				return d.ArgErr()
			}
			m.PolicyFile = d.Val()

		case "difficulty":
			if !d.NextArg() {
				return d.ArgErr()
			}
			var n int
			if _, err := fmt.Sscanf(d.Val(), "%d", &n); err != nil {
				return d.Errf("invalid difficulty %q: %v", d.Val(), err)
			}
			m.Difficulty = n

		case "cookie_domain":
			if !d.NextArg() {
				return d.ArgErr()
			}
			m.CookieDomain = d.Val()

		case "cookie_dynamic_domain":
			m.CookieDynamicDomain = true

		case "cookie_expiration":
			if !d.NextArg() {
				return d.ArgErr()
			}
			dur, err := time.ParseDuration(d.Val())
			if err != nil {
				return d.Errf("invalid cookie_expiration %q: %v", d.Val(), err)
			}
			m.CookieExpiration = caddy.Duration(dur)

		case "cookie_insecure":
			f := false
			m.CookieSecure = &f

		case "cookie_same_site":
			if !d.NextArg() {
				return d.ArgErr()
			}
			m.CookieSameSite = d.Val()

		case "cookie_partitioned":
			m.CookiePartitioned = true

		case "base_prefix":
			if !d.NextArg() {
				return d.ArgErr()
			}
			m.BasePrefix = d.Val()

		case "strip_base_prefix":
			m.StripBasePrefix = true

		case "redirect_domains":
			m.RedirectDomains = append(m.RedirectDomains, d.RemainingArgs()...)
			if len(m.RedirectDomains) == 0 {
				return d.ArgErr()
			}

		case "webmaster_email":
			if !d.NextArg() {
				return d.ArgErr()
			}
			m.WebmasterEmail = d.Val()

		case "serve_robots_txt":
			m.ServeRobotsTXT = true

		case "public_url":
			if !d.NextArg() {
				return d.ArgErr()
			}
			m.PublicURL = d.Val()

		case "hs512_secret":
			if !d.NextArg() {
				return d.ArgErr()
			}
			m.HS512Secret = d.Val()

		case "ed25519_private_key_hex":
			if !d.NextArg() {
				return d.ArgErr()
			}
			m.ED25519PrivateKeyHex = d.Val()

		case "jwt_restriction_header":
			if !d.NextArg() {
				return d.ArgErr()
			}
			m.JWTRestrictionHeader = d.Val()

		case "difficulty_in_jwt":
			m.DifficultyInJWT = true

		case "use_remote_addr":
			m.UseRemoteAddr = true

		default:
			return d.Errf("unknown anubis subdirective %q", d.Val())
		}
	}
	return nil
}

// parseCaddyfile is the httpcaddyfile handler shim.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	m := new(Middleware)
	if err := m.UnmarshalCaddyfile(h.Dispenser); err != nil {
		return nil, err
	}
	return m, nil
}

// parseSameSite converts a string to an http.SameSite value. Defaults to
// SameSiteNoneMode for an empty or unrecognised string.
func parseSameSite(s string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "default":
		return http.SameSiteDefaultMode
	default: // "none" or ""
		return http.SameSiteNoneMode
	}
}

// keyFromHex decodes a hex-encoded ED25519 private key seed (32 bytes).
func keyFromHex(h string) (ed25519.PrivateKey, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, fmt.Errorf("not valid hex: %w", err)
	}
	if len(b) != ed25519.SeedSize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.SeedSize, len(b))
	}
	return ed25519.NewKeyFromSeed(b), nil
}

// Interface assertions – fail fast at compile time if the types are wrong.
var (
	_ caddy.Module                = (*Middleware)(nil)
	_ caddy.Provisioner           = (*Middleware)(nil)
	_ caddy.Validator             = (*Middleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*Middleware)(nil)
	_ caddyfile.Unmarshaler       = (*Middleware)(nil)
)

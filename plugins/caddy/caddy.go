// Package caddy provides a native Caddy HTTP middleware for Anubis.
package caddy

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis"
	libanubis "github.com/TecharoHQ/anubis/lib"

	caddyserver "github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap/exp/zapslog"

	// Register all Anubis store backends for policy files.
	_ "github.com/TecharoHQ/anubis/lib/store/all"
)

func init() {
	caddyserver.RegisterModule(Handler{})
	httpcaddyfile.RegisterHandlerDirective("anubis", parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder("anubis", httpcaddyfile.After, "templates")
}

type nextContextKey struct{}

type nextState struct {
	next caddyhttp.Handler
	err  error
}

type nextHandler struct{}

func (nextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	state, ok := r.Context().Value(nextContextKey{}).(*nextState)
	if !ok || state == nil || state.next == nil {
		return
	}
	state.err = state.next.ServeHTTP(w, r)
}

// Handler is an Anubis middleware module for Caddy.
type Handler struct {
	// PolicyFile is the path to an Anubis bot policy YAML file. The built-in
	// policy is used when this is empty.
	PolicyFile string `json:"policy_file,omitempty"`

	// Difficulty is the default proof-of-work difficulty.
	Difficulty int `json:"difficulty,omitempty"`

	// LogLevel configures Anubis' slog level while loading the policy.
	LogLevel string `json:"log_level,omitempty"`

	CookieDomain        string               `json:"cookie_domain,omitempty"`
	CookieDynamicDomain bool                 `json:"cookie_dynamic_domain,omitempty"`
	CookieExpiration    caddyserver.Duration `json:"cookie_expiration,omitempty"`
	CookieSecure        *bool                `json:"cookie_secure,omitempty"`
	CookieSameSite      string               `json:"cookie_same_site,omitempty"`
	CookiePartitioned   bool                 `json:"cookie_partitioned,omitempty"`

	BasePrefix      string `json:"base_prefix,omitempty"`
	StripBasePrefix bool   `json:"strip_base_prefix,omitempty"`

	RedirectDomains []string `json:"redirect_domains,omitempty"`
	WebmasterEmail  string   `json:"webmaster_email,omitempty"`
	ServeRobotsTXT  bool     `json:"serve_robots_txt,omitempty"`
	PublicURL       string   `json:"public_url,omitempty"`

	HS512Secret              string `json:"hs512_secret,omitempty"`
	ED25519PrivateKeyHex     string `json:"ed25519_private_key_hex,omitempty"`
	JWTRestrictionHeader     string `json:"jwt_restriction_header,omitempty"`
	DifficultyInJWT          bool   `json:"difficulty_in_jwt,omitempty"`
	UseRemoteAddr            bool   `json:"use_remote_addr,omitempty"`
	UseSimplifiedExplanation bool   `json:"use_simplified_explanation,omitempty"`
	ForcedLanguage           string `json:"forced_language,omitempty"`

	server *libanubis.Server
}

// CaddyModule implements caddy.Module.
func (Handler) CaddyModule() caddyserver.ModuleInfo {
	return caddyserver.ModuleInfo{
		ID:  "http.handlers.anubis",
		New: func() caddyserver.Module { return new(Handler) },
	}
}

// Provision implements caddy.Provisioner.
func (h *Handler) Provision(ctx caddyserver.Context) error {
	if err := h.validateConfig(); err != nil {
		return err
	}

	difficulty := h.Difficulty
	if difficulty == 0 {
		difficulty = anubis.DefaultDifficulty
	}

	logLevel := h.LogLevel
	if logLevel == "" {
		logLevel = "INFO"
	}

	policy, err := libanubis.LoadPoliciesOrDefault(context.Background(), h.PolicyFile, difficulty, logLevel, false)
	if err != nil {
		return fmt.Errorf("anubis caddy: load policy: %w", err)
	}

	logger := slog.New(zapslog.NewHandler(ctx.Logger().Core())).With("subsystem", "anubis", "adapter", "caddy")
	anubis.ForcedLanguage = h.ForcedLanguage
	anubis.UseSimplifiedExplanation = h.UseSimplifiedExplanation

	opts := libanubis.Options{
		Next:                 nextHandler{},
		Policy:               policy,
		CookieDomain:         h.CookieDomain,
		CookieDynamicDomain:  h.CookieDynamicDomain,
		CookieExpiration:     time.Duration(h.CookieExpiration),
		CookiePartitioned:    h.CookiePartitioned,
		BasePrefix:           h.BasePrefix,
		StripBasePrefix:      h.StripBasePrefix,
		RedirectDomains:      h.RedirectDomains,
		WebmasterEmail:       h.WebmasterEmail,
		ServeRobotsTXT:       h.ServeRobotsTXT,
		PublicUrl:            h.PublicURL,
		JWTRestrictionHeader: h.JWTRestrictionHeader,
		DifficultyInJWT:      h.DifficultyInJWT,
		Logger:               logger,
		CookieSecure:         true,
		CookieSameSite:       http.SameSiteNoneMode,
	}

	if opts.CookieExpiration == 0 {
		opts.CookieExpiration = anubis.CookieDefaultExpirationTime
	}
	if h.CookieSecure != nil {
		opts.CookieSecure = *h.CookieSecure
	}
	if h.CookieSameSite != "" {
		opts.CookieSameSite, err = parseSameSite(h.CookieSameSite)
		if err != nil {
			return err
		}
	}
	if opts.JWTRestrictionHeader == "" {
		opts.JWTRestrictionHeader = "X-Real-IP"
	}
	if h.ED25519PrivateKeyHex != "" {
		opts.ED25519PrivateKey, err = keyFromHex(h.ED25519PrivateKeyHex)
		if err != nil {
			return fmt.Errorf("anubis caddy: parse ed25519_private_key_hex: %w", err)
		}
	}
	if h.HS512Secret != "" {
		opts.HS512Secret = []byte(h.HS512Secret)
	}

	h.server, err = libanubis.New(opts)
	if err != nil {
		return fmt.Errorf("anubis caddy: create server: %w", err)
	}
	return nil
}

// Validate implements caddy.Validator.
func (h *Handler) Validate() error {
	if h.server == nil {
		return errors.New("anubis caddy: server is not provisioned")
	}
	return h.validateConfig()
}

func (h *Handler) validateConfig() error {
	var errs []error
	if h.Difficulty < 0 || h.Difficulty > 64 {
		errs = append(errs, fmt.Errorf("anubis caddy: difficulty must be between 0 and 64, got %d", h.Difficulty))
	}
	if h.CookieDomain != "" && h.CookieDynamicDomain {
		errs = append(errs, errors.New("anubis caddy: cookie_domain and cookie_dynamic_domain are mutually exclusive"))
	}
	if h.BasePrefix != "" {
		if !strings.HasPrefix(h.BasePrefix, "/") {
			errs = append(errs, fmt.Errorf("anubis caddy: base_prefix must start with /, got %q", h.BasePrefix))
		}
		if strings.HasSuffix(h.BasePrefix, "/") {
			errs = append(errs, fmt.Errorf("anubis caddy: base_prefix must not end with /, got %q", h.BasePrefix))
		}
	}
	if h.StripBasePrefix && h.BasePrefix == "" {
		errs = append(errs, errors.New("anubis caddy: strip_base_prefix requires base_prefix"))
	}
	if h.HS512Secret != "" && h.ED25519PrivateKeyHex != "" {
		errs = append(errs, errors.New("anubis caddy: hs512_secret and ed25519_private_key_hex are mutually exclusive"))
	}
	if h.CookieSameSite != "" {
		if _, err := parseSameSite(h.CookieSameSite); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if h.server == nil {
		return errors.New("anubis caddy: server is not provisioned")
	}
	if h.UseRemoteAddr && r.Header.Get("X-Real-Ip") == "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		r = r.Clone(r.Context())
		r.Header.Set("X-Real-Ip", host)
	}

	state := &nextState{next: next}
	r = r.WithContext(context.WithValue(r.Context(), nextContextKey{}, state))
	h.server.ServeHTTP(w, r)
	return state.err
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
//
//	anubis [<policy_file>] {
//	    policy_file <path>
//	    difficulty <n>
//	    log_level <level>
//	    cookie_domain <domain>
//	    cookie_dynamic_domain
//	    cookie_expiration <duration>
//	    cookie_secure <bool>
//	    cookie_insecure
//	    cookie_same_site <None|Lax|Strict|Default>
//	    cookie_partitioned
//	    base_prefix <prefix>
//	    strip_base_prefix
//	    redirect_domains <domain> [<domain> ...]
//	    webmaster_email <email>
//	    serve_robots_txt
//	    public_url <url>
//	    hs512_secret <secret>
//	    ed25519_private_key_hex <hex>
//	    jwt_restriction_header <header>
//	    difficulty_in_jwt
//	    use_remote_addr
//	    forced_language <language>
//	    use_simplified_explanation
//	}
func (h *Handler) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next()
	if d.NextArg() {
		h.PolicyFile = d.Val()
		if d.NextArg() {
			return d.ArgErr()
		}
	}

	for d.NextBlock(0) {
		switch d.Val() {
		case "policy_file":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.PolicyFile = d.Val()
		case "difficulty":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			n, err := strconv.Atoi(d.Val())
			if err != nil {
				return d.Errf("invalid difficulty %q: %v", d.Val(), err)
			}
			h.Difficulty = n
		case "log_level":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.LogLevel = d.Val()
		case "cookie_domain":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.CookieDomain = d.Val()
		case "cookie_dynamic_domain":
			if err := noArgs(d); err != nil {
				return err
			}
			h.CookieDynamicDomain = true
		case "cookie_expiration":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			dur, err := caddyserver.ParseDuration(d.Val())
			if err != nil {
				return d.Errf("invalid cookie_expiration %q: %v", d.Val(), err)
			}
			h.CookieExpiration = caddyserver.Duration(dur)
		case "cookie_secure":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			secure, err := strconv.ParseBool(d.Val())
			if err != nil {
				return d.Errf("invalid cookie_secure %q: %v", d.Val(), err)
			}
			h.CookieSecure = &secure
		case "cookie_insecure":
			if err := noArgs(d); err != nil {
				return err
			}
			secure := false
			h.CookieSecure = &secure
		case "cookie_same_site":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.CookieSameSite = d.Val()
		case "cookie_partitioned":
			if err := noArgs(d); err != nil {
				return err
			}
			h.CookiePartitioned = true
		case "base_prefix":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.BasePrefix = d.Val()
		case "strip_base_prefix":
			if err := noArgs(d); err != nil {
				return err
			}
			h.StripBasePrefix = true
		case "redirect_domains":
			h.RedirectDomains = append(h.RedirectDomains, d.RemainingArgs()...)
			if len(h.RedirectDomains) == 0 {
				return d.ArgErr()
			}
		case "webmaster_email":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.WebmasterEmail = d.Val()
		case "serve_robots_txt":
			if err := noArgs(d); err != nil {
				return err
			}
			h.ServeRobotsTXT = true
		case "public_url":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.PublicURL = d.Val()
		case "hs512_secret":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.HS512Secret = d.Val()
		case "ed25519_private_key_hex":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.ED25519PrivateKeyHex = d.Val()
		case "jwt_restriction_header":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.JWTRestrictionHeader = d.Val()
		case "difficulty_in_jwt":
			if err := noArgs(d); err != nil {
				return err
			}
			h.DifficultyInJWT = true
		case "use_remote_addr":
			if err := noArgs(d); err != nil {
				return err
			}
			h.UseRemoteAddr = true
		case "forced_language":
			if err := nextSingleArg(d); err != nil {
				return err
			}
			h.ForcedLanguage = d.Val()
		case "use_simplified_explanation":
			if err := noArgs(d); err != nil {
				return err
			}
			h.UseSimplifiedExplanation = true
		default:
			return d.Errf("unknown anubis subdirective %q", d.Val())
		}
	}
	return h.validateConfig()
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m Handler
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return &m, err
}

func nextSingleArg(d *caddyfile.Dispenser) error {
	if !d.NextArg() {
		return d.ArgErr()
	}
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

func noArgs(d *caddyfile.Dispenser) error {
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

func parseSameSite(value string) (http.SameSite, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none":
		return http.SameSiteNoneMode, nil
	case "lax":
		return http.SameSiteLaxMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "default":
		return http.SameSiteDefaultMode, nil
	default:
		return http.SameSiteDefaultMode, fmt.Errorf("anubis caddy: invalid cookie_same_site %q, valid values are None, Lax, Strict, and Default", value)
	}
}

func keyFromHex(value string) (ed25519.PrivateKey, error) {
	keyBytes, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("supplied key is not hex-encoded: %w", err)
	}
	if len(keyBytes) != ed25519.SeedSize {
		return nil, fmt.Errorf("supplied key is not %d bytes long, got %d bytes", ed25519.SeedSize, len(keyBytes))
	}
	return ed25519.NewKeyFromSeed(keyBytes), nil
}

var (
	_ caddyserver.Module          = (*Handler)(nil)
	_ caddyserver.Provisioner     = (*Handler)(nil)
	_ caddyserver.Validator       = (*Handler)(nil)
	_ caddyhttp.MiddlewareHandler = (*Handler)(nil)
	_ caddyfile.Unmarshaler       = (*Handler)(nil)
)

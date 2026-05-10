package caddy

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/web"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func TestUnmarshalCaddyfile(t *testing.T) {
	secure := false
	d := caddyfile.NewTestDispenser(`
anubis /etc/anubis/policy.yaml {
	difficulty 5
	log_level DEBUG
	cookie_domain example.com
	cookie_expiration 2h
	cookie_insecure
	cookie_same_site Lax
	cookie_partitioned
	base_prefix /gate
	strip_base_prefix
	redirect_domains example.com *.example.com
	webmaster_email webmaster@example.com
	serve_robots_txt
	public_url https://anubis.example.com
	hs512_secret testing
	jwt_restriction_header X-Forwarded-For
	difficulty_in_jwt
	use_remote_addr
	forced_language en
	use_simplified_explanation
}
`)
	var h Handler
	if err := h.UnmarshalCaddyfile(d); err != nil {
		t.Fatalf("UnmarshalCaddyfile returned error: %v", err)
	}

	if h.PolicyFile != "/etc/anubis/policy.yaml" {
		t.Fatalf("PolicyFile = %q", h.PolicyFile)
	}
	if h.Difficulty != 5 {
		t.Fatalf("Difficulty = %d", h.Difficulty)
	}
	if h.LogLevel != "DEBUG" {
		t.Fatalf("LogLevel = %q", h.LogLevel)
	}
	if h.CookieDomain != "example.com" {
		t.Fatalf("CookieDomain = %q", h.CookieDomain)
	}
	if time.Duration(h.CookieExpiration) != 2*time.Hour {
		t.Fatalf("CookieExpiration = %s", time.Duration(h.CookieExpiration))
	}
	if h.CookieSecure == nil || *h.CookieSecure != secure {
		t.Fatalf("CookieSecure = %v", h.CookieSecure)
	}
	if h.CookieSameSite != "Lax" {
		t.Fatalf("CookieSameSite = %q", h.CookieSameSite)
	}
	if !h.CookiePartitioned || !h.StripBasePrefix || !h.ServeRobotsTXT || !h.DifficultyInJWT || !h.UseRemoteAddr || !h.UseSimplifiedExplanation {
		t.Fatalf("expected boolean flags to be true: %#v", h)
	}
	if h.BasePrefix != "/gate" {
		t.Fatalf("BasePrefix = %q", h.BasePrefix)
	}
	if strings.Join(h.RedirectDomains, ",") != "example.com,*.example.com" {
		t.Fatalf("RedirectDomains = %#v", h.RedirectDomains)
	}
	if h.WebmasterEmail != "webmaster@example.com" || h.PublicURL != "https://anubis.example.com" {
		t.Fatalf("unexpected contact/public URL: %#v", h)
	}
	if h.HS512Secret != "testing" || h.JWTRestrictionHeader != "X-Forwarded-For" || h.ForcedLanguage != "en" {
		t.Fatalf("unexpected secret/header/language fields: %#v", h)
	}
}

func TestUnmarshalCaddyfileRejectsInvalidConfig(t *testing.T) {
	tests := []string{
		`anubis {
			difficulty -1
		}`,
		`anubis {
			cookie_domain example.com
			cookie_dynamic_domain
		}`,
		`anubis {
			base_prefix gate
		}`,
		`anubis {
			base_prefix /gate/
		}`,
		`anubis {
			strip_base_prefix
		}`,
		`anubis {
			cookie_same_site sideways
		}`,
		`anubis {
			hs512_secret secret
			ed25519_private_key_hex 0000000000000000000000000000000000000000000000000000000000000000
		}`,
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			var h Handler
			if err := h.UnmarshalCaddyfile(caddyfile.NewTestDispenser(input)); err == nil {
				t.Fatalf("expected invalid config to fail")
			}
		})
	}
}

func TestParseSameSite(t *testing.T) {
	tests := map[string]http.SameSite{
		"":         http.SameSiteNoneMode,
		"none":     http.SameSiteNoneMode,
		"Lax":      http.SameSiteLaxMode,
		" strict ": http.SameSiteStrictMode,
		"default":  http.SameSiteDefaultMode,
	}
	for input, want := range tests {
		got, err := parseSameSite(input)
		if err != nil {
			t.Fatalf("parseSameSite(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("parseSameSite(%q) = %v, want %v", input, got, want)
		}
	}
	if _, err := parseSameSite("invalid"); err == nil {
		t.Fatal("expected invalid SameSite mode to fail")
	}
}

func TestKeyFromHex(t *testing.T) {
	const seed = "0000000000000000000000000000000000000000000000000000000000000000"
	if key, err := keyFromHex(seed); err != nil || len(key) == 0 {
		t.Fatalf("keyFromHex(valid) = len %d, err %v", len(key), err)
	}
	if _, err := keyFromHex("not-hex"); err == nil {
		t.Fatal("expected invalid hex to fail")
	}
	if _, err := keyFromHex("00"); err == nil {
		t.Fatal("expected short seed to fail")
	}
}

func TestNextHandlerPropagatesCaddyError(t *testing.T) {
	want := errors.New("next failed")
	state := &nextState{next: caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error {
		return want
	})}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), nextContextKey{}, state))

	nextHandler{}.ServeHTTP(httptest.NewRecorder(), req)

	if !errors.Is(state.err, want) {
		t.Fatalf("state.err = %v, want %v", state.err, want)
	}
}

func TestCaddyfileDirectiveRunsBeforeRespond(t *testing.T) {
	adapter := caddyfile.Adapter{ServerType: httpcaddyfile.ServerType{}}
	out, _, err := adapter.Adapt([]byte(`:8080 {
	anubis
	respond "hello"
}`), nil)
	if err != nil {
		t.Fatalf("Adapt returned error: %v", err)
	}

	config := string(out)
	anubisIndex := strings.Index(config, `"handler":"anubis"`)
	respondIndex := strings.Index(config, `"handler":"static_response"`)
	if anubisIndex == -1 {
		t.Fatalf("adapted config does not contain anubis handler: %s", config)
	}
	if respondIndex == -1 {
		t.Fatalf("adapted config does not contain respond handler: %s", config)
	}
	if anubisIndex > respondIndex {
		t.Fatalf("anubis handler must run before respond; config: %s", config)
	}
}

func TestEmbeddedRuntimeAssetsPresent(t *testing.T) {
	for _, name := range []string{
		"static/js/main.mjs",
		"static/js/bench.mjs",
		"static/js/worker/sha256-purejs.mjs",
		"static/js/worker/sha256-webcrypto.mjs",
		"static/locales/en.json",
		"static/locales/manifest.json",
	} {
		t.Run(name, func(t *testing.T) {
			info, err := fs.Stat(web.Static, name)
			if err != nil {
				t.Fatalf("expected embedded asset %q: %v", name, err)
			}
			if info.Size() == 0 {
				t.Fatalf("embedded asset %q is empty", name)
			}
		})
	}
}

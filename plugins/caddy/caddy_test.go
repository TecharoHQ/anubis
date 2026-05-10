package caddy

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/web"
	caddyserver "github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddyserver.RegisterModule(testReverseProxyHandler{})
	caddyserver.RegisterModule(testFileServerHandler{})
	httpcaddyfile.RegisterHandlerDirective("reverse_proxy", parseTestCaddyfileReverseProxy)
	httpcaddyfile.RegisterHandlerDirective("file_server", parseTestCaddyfileFileServer)
}

type testReverseProxyHandler struct{}

func (testReverseProxyHandler) CaddyModule() caddyserver.ModuleInfo {
	return caddyserver.ModuleInfo{
		ID:  "http.handlers.reverse_proxy",
		New: func() caddyserver.Module { return new(testReverseProxyHandler) },
	}
}

func (testReverseProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return next.ServeHTTP(w, r)
}

type testFileServerHandler struct{}

func (testFileServerHandler) CaddyModule() caddyserver.ModuleInfo {
	return caddyserver.ModuleInfo{
		ID:  "http.handlers.file_server",
		New: func() caddyserver.Module { return new(testFileServerHandler) },
	}
}

func (testFileServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	return next.ServeHTTP(w, r)
}

func parseTestCaddyfileReverseProxy(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	if err := consumeTestCaddyfileArgs(h); err != nil {
		return nil, err
	}
	return testReverseProxyHandler{}, nil
}

func parseTestCaddyfileFileServer(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	if err := consumeTestCaddyfileArgs(h); err != nil {
		return nil, err
	}
	return testFileServerHandler{}, nil
}

func consumeTestCaddyfileArgs(h httpcaddyfile.Helper) error {
	if !h.Next() {
		return h.ArgErr()
	}
	for h.NextArg() {
	}
	for h.NextBlock(0) {
		for h.NextArg() {
		}
	}
	return nil
}

func TestUnmarshalCaddyfile(t *testing.T) {
	secure := false
	d := caddyfile.NewTestDispenser(`
anubis {
	policy_file /etc/anubis/policy.yaml
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
		`anubis {
			use_remote_addr maybe
		}`,
		`anubis {
			serve_robots_txt true false
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

func TestUnmarshalCaddyfileOptionalBooleanFalseValues(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
anubis {
	cookie_dynamic_domain false
	cookie_partitioned false
	strip_base_prefix false
	serve_robots_txt false
	difficulty_in_jwt false
	use_remote_addr false
	use_simplified_explanation false
}
`)
	var h Handler
	if err := h.UnmarshalCaddyfile(d); err != nil {
		t.Fatalf("UnmarshalCaddyfile returned error: %v", err)
	}

	for _, field := range []string{
		"cookie_dynamic_domain",
		"cookie_partitioned",
		"strip_base_prefix",
		"serve_robots_txt",
		"difficulty_in_jwt",
		"use_remote_addr",
		"use_simplified_explanation",
	} {
		if !h.isCaddyfileFieldSet(field) {
			t.Fatalf("expected %s to be marked as explicitly set", field)
		}
	}
	if h.CookieDynamicDomain || h.CookiePartitioned || h.StripBasePrefix || h.ServeRobotsTXT || h.DifficultyInJWT || h.UseRemoteAddr || h.UseSimplifiedExplanation {
		t.Fatalf("optional boolean false values were not preserved: %#v", h)
	}
}

func TestCaddyfileGlobalDefaults(t *testing.T) {
	adapter := caddyfile.Adapter{ServerType: httpcaddyfile.ServerType{}}
	out, _, err := adapter.Adapt([]byte(`{
	anubis {
		difficulty 7
		cookie_domain example.com
		public_url https://anubis.example.com
		use_remote_addr
	}
}

:8080 {
	anubis {
		difficulty 0
		use_remote_addr false
	}
	respond "hello"
}`), nil)
	if err != nil {
		t.Fatalf("Adapt returned error: %v", err)
	}

	config := string(out)
	for _, want := range []string{
		`"handler":"anubis"`,
		`"cookie_domain":"example.com"`,
		`"public_url":"https://anubis.example.com"`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("adapted config does not contain %s: %s", want, config)
		}
	}
	for _, unexpected := range []string{
		`"difficulty"`,
		`"use_remote_addr":true`,
	} {
		if strings.Contains(config, unexpected) {
			t.Fatalf("local value should override global default; found %s in config: %s", unexpected, config)
		}
	}
}

func TestCaddyfileGlobalDefaultsSatisfyLocalValidation(t *testing.T) {
	adapter := caddyfile.Adapter{ServerType: httpcaddyfile.ServerType{}}
	out, _, err := adapter.Adapt([]byte(`{
	anubis {
		base_prefix /gate
	}
}

:8080 {
	anubis {
		strip_base_prefix
	}
	respond "hello"
}`), nil)
	if err != nil {
		t.Fatalf("Adapt returned error: %v", err)
	}

	config := string(out)
	for _, want := range []string{
		`"base_prefix":"/gate"`,
		`"strip_base_prefix":true`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("adapted config does not contain %s: %s", want, config)
		}
	}
}

func TestCaddyfileLocalConfigOverridesMutuallyExclusiveGlobalDefault(t *testing.T) {
	adapter := caddyfile.Adapter{ServerType: httpcaddyfile.ServerType{}}
	out, _, err := adapter.Adapt([]byte(`{
	anubis {
		cookie_domain example.com
	}
}

:8080 {
	anubis {
		cookie_dynamic_domain
	}
	respond "hello"
}`), nil)
	if err != nil {
		t.Fatalf("Adapt returned error: %v", err)
	}

	config := string(out)
	if !strings.Contains(config, `"cookie_dynamic_domain":true`) {
		t.Fatalf("adapted config does not contain cookie_dynamic_domain: %s", config)
	}
	if strings.Contains(config, `"cookie_domain"`) {
		t.Fatalf("local cookie_dynamic_domain should override global cookie_domain; config: %s", config)
	}
}

func TestCaddyfileMatcherToken(t *testing.T) {
	adapter := caddyfile.Adapter{ServerType: httpcaddyfile.ServerType{}}
	out, _, err := adapter.Adapt([]byte(`:8080 {
	@admin path /admin/*
	anubis @admin {
		policy_file /etc/anubis/policy.yaml
		difficulty 5
	}
	respond "hello"
}`), nil)
	if err != nil {
		t.Fatalf("Adapt returned error: %v", err)
	}

	config := string(out)
	for _, want := range []string{
		`"path":["/admin/*"]`,
		`"handler":"anubis"`,
		`"policy_file":"/etc/anubis/policy.yaml"`,
		`"difficulty":5`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("adapted config does not contain %s: %s", want, config)
		}
	}
}

func TestCaddyfileAllowsPlaceholdersInValidatedStrings(t *testing.T) {
	adapter := caddyfile.Adapter{ServerType: httpcaddyfile.ServerType{}}
	out, _, err := adapter.Adapt([]byte(`:8080 {
	anubis {
		base_prefix {env.ANUBIS_CADDY_BASE_PREFIX}
		cookie_same_site {env.ANUBIS_CADDY_SAME_SITE}
	}
	respond "hello"
}`), nil)
	if err != nil {
		t.Fatalf("Adapt returned error: %v", err)
	}

	config := string(out)
	for _, want := range []string{
		`"base_prefix":"{env.ANUBIS_CADDY_BASE_PREFIX}"`,
		`"cookie_same_site":"{env.ANUBIS_CADDY_SAME_SITE}"`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("adapted config does not contain %s: %s", want, config)
		}
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

func TestProvisionRestoresAnubisGlobals(t *testing.T) {
	origBasePrefix := anubis.BasePrefix
	origPublicURL := anubis.PublicUrl
	origForcedLanguage := anubis.ForcedLanguage
	origUseSimplifiedExplanation := anubis.UseSimplifiedExplanation
	t.Cleanup(func() {
		anubis.BasePrefix = origBasePrefix
		anubis.PublicUrl = origPublicURL
		anubis.ForcedLanguage = origForcedLanguage
		anubis.UseSimplifiedExplanation = origUseSimplifiedExplanation
	})

	anubis.BasePrefix = "/existing"
	anubis.PublicUrl = "https://existing.example"
	anubis.ForcedLanguage = "de"
	anubis.UseSimplifiedExplanation = true

	ctx, cancel := caddyserver.NewContext(caddyserver.Context{Context: context.Background()})
	defer cancel()

	h := Handler{
		BasePrefix:               "/gate",
		PublicURL:                "https://anubis.example.com",
		ForcedLanguage:           "en",
		UseSimplifiedExplanation: false,
		CookieDomain:             "example.com",
		JWTRestrictionHeader:     "X-Real-IP",
		ED25519PrivateKeyHex:     "0000000000000000000000000000000000000000000000000000000000000000",
	}
	if err := h.Provision(ctx); err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}

	if anubis.BasePrefix != "/existing" {
		t.Fatalf("BasePrefix leaked after Provision: %q", anubis.BasePrefix)
	}
	if anubis.PublicUrl != "https://existing.example" {
		t.Fatalf("PublicUrl leaked after Provision: %q", anubis.PublicUrl)
	}
	if anubis.ForcedLanguage != "de" {
		t.Fatalf("ForcedLanguage leaked after Provision: %q", anubis.ForcedLanguage)
	}
	if !anubis.UseSimplifiedExplanation {
		t.Fatal("UseSimplifiedExplanation leaked after Provision")
	}
}

func TestProvisionReplacesCaddyPlaceholders(t *testing.T) {
	policyFile, err := filepath.Abs(filepath.Join("..", "..", "lib", "testdata", "permissive.yaml"))
	if err != nil {
		t.Fatalf("Abs returned error: %v", err)
	}
	t.Setenv("ANUBIS_CADDY_POLICY", policyFile)
	t.Setenv("ANUBIS_CADDY_KEY", "0000000000000000000000000000000000000000000000000000000000000000")
	t.Setenv("ANUBIS_CADDY_DOMAIN", "example.com")
	t.Setenv("ANUBIS_CADDY_HEADER", "X-Forwarded-For")
	t.Setenv("ANUBIS_CADDY_BASE_PREFIX", "/gate")
	t.Setenv("ANUBIS_CADDY_SAME_SITE", "Lax")

	ctx, cancel := caddyserver.NewContext(caddyserver.Context{Context: context.Background()})
	defer cancel()

	h := Handler{
		PolicyFile:           "{env.ANUBIS_CADDY_POLICY}",
		CookieDomain:         "{env.ANUBIS_CADDY_DOMAIN}",
		CookieSameSite:       "{env.ANUBIS_CADDY_SAME_SITE}",
		BasePrefix:           "{env.ANUBIS_CADDY_BASE_PREFIX}",
		ED25519PrivateKeyHex: "{env.ANUBIS_CADDY_KEY}",
		JWTRestrictionHeader: "{env.ANUBIS_CADDY_HEADER}",
	}
	if err := h.Provision(ctx); err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := h.Cleanup(); err != nil {
			t.Fatalf("Cleanup returned error: %v", err)
		}
	})

	if h.PolicyFile != policyFile {
		t.Fatalf("PolicyFile = %q, want %q", h.PolicyFile, policyFile)
	}
	if h.CookieDomain != "example.com" {
		t.Fatalf("CookieDomain = %q", h.CookieDomain)
	}
	if h.CookieSameSite != "Lax" {
		t.Fatalf("CookieSameSite = %q", h.CookieSameSite)
	}
	if h.BasePrefix != "/gate" {
		t.Fatalf("BasePrefix = %q", h.BasePrefix)
	}
	if h.ED25519PrivateKeyHex != "0000000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("ED25519PrivateKeyHex = %q", h.ED25519PrivateKeyHex)
	}
	if h.JWTRestrictionHeader != "X-Forwarded-For" {
		t.Fatalf("JWTRestrictionHeader = %q", h.JWTRestrictionHeader)
	}
}

func TestCleanupClosesPolicyResources(t *testing.T) {
	store := &closeTrackingStore{}
	canceled := false
	h := Handler{
		policyCancel: func() { canceled = true },
		policyStore:  store,
	}

	if err := h.Cleanup(); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if !canceled {
		t.Fatal("expected policy cancel function to be called")
	}
	if !store.closed {
		t.Fatal("expected policy store to be closed")
	}
	if h.policyCancel != nil {
		t.Fatal("policyCancel was not cleared")
	}
	if h.policyStore != nil {
		t.Fatal("policyStore was not cleared")
	}
}

func TestWithAnubisGlobalsRestoresAfterError(t *testing.T) {
	origBasePrefix := anubis.BasePrefix
	origPublicURL := anubis.PublicUrl
	origForcedLanguage := anubis.ForcedLanguage
	origUseSimplifiedExplanation := anubis.UseSimplifiedExplanation
	t.Cleanup(func() {
		anubis.BasePrefix = origBasePrefix
		anubis.PublicUrl = origPublicURL
		anubis.ForcedLanguage = origForcedLanguage
		anubis.UseSimplifiedExplanation = origUseSimplifiedExplanation
	})

	anubis.BasePrefix = "/before"
	anubis.PublicUrl = "https://before.example"
	anubis.ForcedLanguage = "fr"
	anubis.UseSimplifiedExplanation = false

	want := errors.New("stop")
	h := Handler{
		BasePrefix:               "/during",
		PublicURL:                "https://during.example",
		ForcedLanguage:           "en",
		UseSimplifiedExplanation: true,
	}
	err := h.withAnubisGlobals(func() error {
		if anubis.BasePrefix != "/during" {
			t.Fatalf("BasePrefix inside callback = %q", anubis.BasePrefix)
		}
		if anubis.PublicUrl != "https://during.example" {
			t.Fatalf("PublicUrl inside callback = %q", anubis.PublicUrl)
		}
		if anubis.ForcedLanguage != "en" {
			t.Fatalf("ForcedLanguage inside callback = %q", anubis.ForcedLanguage)
		}
		if !anubis.UseSimplifiedExplanation {
			t.Fatal("UseSimplifiedExplanation inside callback = false")
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("withAnubisGlobals error = %v, want %v", err, want)
	}

	if anubis.BasePrefix != "/before" {
		t.Fatalf("BasePrefix after callback = %q", anubis.BasePrefix)
	}
	if anubis.PublicUrl != "https://before.example" {
		t.Fatalf("PublicUrl after callback = %q", anubis.PublicUrl)
	}
	if anubis.ForcedLanguage != "fr" {
		t.Fatalf("ForcedLanguage after callback = %q", anubis.ForcedLanguage)
	}
	if anubis.UseSimplifiedExplanation {
		t.Fatal("UseSimplifiedExplanation after callback = true")
	}
}

func TestNextHandlerCapturesAllowedRequest(t *testing.T) {
	state := &nextState{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), nextContextKey{}, state))

	nextHandler{}.ServeHTTP(rec, req)

	if state.w != rec {
		t.Fatalf("state.w = %p, want %p", state.w, rec)
	}
	if state.r != req {
		t.Fatalf("state.r = %p, want %p", state.r, req)
	}
}

func TestServeHTTPRunsNextAfterRestoringAnubisGlobals(t *testing.T) {
	origBasePrefix := anubis.BasePrefix
	origPublicURL := anubis.PublicUrl
	origForcedLanguage := anubis.ForcedLanguage
	origUseSimplifiedExplanation := anubis.UseSimplifiedExplanation
	t.Cleanup(func() {
		anubis.BasePrefix = origBasePrefix
		anubis.PublicUrl = origPublicURL
		anubis.ForcedLanguage = origForcedLanguage
		anubis.UseSimplifiedExplanation = origUseSimplifiedExplanation
	})

	anubis.BasePrefix = "/outside"
	anubis.PublicUrl = "https://outside.example"
	anubis.ForcedLanguage = "de"
	anubis.UseSimplifiedExplanation = true

	ctx, cancel := caddyserver.NewContext(caddyserver.Context{Context: context.Background()})
	defer cancel()

	h := Handler{
		PolicyFile:           filepath.Join("..", "..", "lib", "testdata", "permissive.yaml"),
		BasePrefix:           "/gate",
		PublicURL:            "https://anubis.example.com",
		ForcedLanguage:       "en",
		CookieDomain:         "example.com",
		ED25519PrivateKeyHex: "0000000000000000000000000000000000000000000000000000000000000000",
	}
	if err := h.Provision(ctx); err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	req.Header.Set("X-Real-Ip", "10.0.0.1")
	rec := httptest.NewRecorder()
	nextCalled := false
	next := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		nextCalled = true
		if anubis.BasePrefix != "/outside" {
			t.Fatalf("BasePrefix leaked into next handler: %q", anubis.BasePrefix)
		}
		if anubis.PublicUrl != "https://outside.example" {
			t.Fatalf("PublicUrl leaked into next handler: %q", anubis.PublicUrl)
		}
		if anubis.ForcedLanguage != "de" {
			t.Fatalf("ForcedLanguage leaked into next handler: %q", anubis.ForcedLanguage)
		}
		if !anubis.UseSimplifiedExplanation {
			t.Fatal("UseSimplifiedExplanation leaked into next handler")
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	})

	if err := h.ServeHTTP(rec, req, next); err != nil {
		t.Fatalf("ServeHTTP returned error: %v", err)
	}
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestServeHTTPReturnsNextError(t *testing.T) {
	ctx, cancel := caddyserver.NewContext(caddyserver.Context{Context: context.Background()})
	defer cancel()

	h := Handler{
		PolicyFile:           filepath.Join("..", "..", "lib", "testdata", "permissive.yaml"),
		CookieDomain:         "example.com",
		ED25519PrivateKeyHex: "0000000000000000000000000000000000000000000000000000000000000000",
	}
	if err := h.Provision(ctx); err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}

	want := errors.New("next failed")
	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	req.Header.Set("X-Real-Ip", "10.0.0.1")
	rec := httptest.NewRecorder()
	next := caddyhttp.HandlerFunc(func(http.ResponseWriter, *http.Request) error {
		return want
	})

	if err := h.ServeHTTP(rec, req, next); !errors.Is(err, want) {
		t.Fatalf("ServeHTTP error = %v, want %v", err, want)
	}
}

func TestServeHTTPUseRemoteAddrPopulatesMissingRealIP(t *testing.T) {
	ctx, cancel := caddyserver.NewContext(caddyserver.Context{Context: context.Background()})
	defer cancel()

	h := Handler{
		PolicyFile:           filepath.Join("..", "..", "lib", "testdata", "permissive.yaml"),
		CookieDomain:         "example.com",
		ED25519PrivateKeyHex: "0000000000000000000000000000000000000000000000000000000000000000",
		UseRemoteAddr:        true,
	}
	if err := h.Provision(ctx); err != nil {
		t.Fatalf("Provision returned error: %v", err)
	}

	tests := []struct {
		name           string
		remoteAddr     string
		existingRealIP string
		wantRealIP     string
	}{
		{
			name:       "missing header uses remote address host",
			remoteAddr: "203.0.113.9:12345",
			wantRealIP: "203.0.113.9",
		},
		{
			name:           "existing header is preserved",
			remoteAddr:     "203.0.113.9:12345",
			existingRealIP: "198.51.100.7",
			wantRealIP:     "198.51.100.7",
		},
		{
			name:       "remote address without port is accepted",
			remoteAddr: "2001:db8::1",
			wantRealIP: "2001:db8::1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.existingRealIP != "" {
				req.Header.Set("X-Real-Ip", tc.existingRealIP)
			}
			rec := httptest.NewRecorder()
			next := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				if got := r.Header.Get("X-Real-Ip"); got != tc.wantRealIP {
					t.Fatalf("X-Real-Ip = %q, want %q", got, tc.wantRealIP)
				}
				w.WriteHeader(http.StatusNoContent)
				return nil
			})

			if err := h.ServeHTTP(rec, req, next); err != nil {
				t.Fatalf("ServeHTTP returned error: %v", err)
			}
			if rec.Code != http.StatusNoContent {
				t.Fatalf("response code = %d, want %d", rec.Code, http.StatusNoContent)
			}
		})
	}
}

func TestCaddyfileDirectiveRunsBeforeResponseHandlers(t *testing.T) {
	tests := []struct {
		name              string
		caddyfile         string
		downstreamHandler string
	}{
		{
			name: "respond",
			caddyfile: `:8080 {
	anubis
	respond "hello"
}`,
			downstreamHandler: `"handler":"static_response"`,
		},
		{
			name: "reverse_proxy",
			caddyfile: `:8080 {
	anubis
	reverse_proxy 127.0.0.1:65535
}`,
			downstreamHandler: `"handler":"reverse_proxy"`,
		},
		{
			name: "file_server",
			caddyfile: `:8080 {
	anubis
	file_server
}`,
			downstreamHandler: `"handler":"file_server"`,
		},
	}

	adapter := caddyfile.Adapter{ServerType: httpcaddyfile.ServerType{}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := adapter.Adapt([]byte(tc.caddyfile), nil)
			if err != nil {
				t.Fatalf("Adapt returned error: %v", err)
			}

			config := string(out)
			anubisIndex := strings.Index(config, `"handler":"anubis"`)
			downstreamIndex := strings.Index(config, tc.downstreamHandler)
			if anubisIndex == -1 {
				t.Fatalf("adapted config does not contain anubis handler: %s", config)
			}
			if downstreamIndex == -1 {
				t.Fatalf("adapted config does not contain %s: %s", tc.downstreamHandler, config)
			}
			if anubisIndex > downstreamIndex {
				t.Fatalf("anubis handler must run before %s; config: %s", tc.name, config)
			}
		})
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

type closeTrackingStore struct {
	closed bool
}

func (s *closeTrackingStore) Delete(context.Context, string) error {
	return nil
}

func (s *closeTrackingStore) Get(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (s *closeTrackingStore) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (s *closeTrackingStore) IsPersistent() bool {
	return false
}

func (s *closeTrackingStore) Close() {
	s.closed = true
}

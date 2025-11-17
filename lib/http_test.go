package lib

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/policy"
)

func TestSetCookie(t *testing.T) {
	for _, tt := range []struct {
		name       string
		host       string
		cookieName string
		options    Options
	}{
		{
			name:       "basic",
			options:    Options{},
			host:       "",
			cookieName: anubis.CookieName,
		},
		{
			name:       "domain techaro.lol",
			options:    Options{CookieDomain: "techaro.lol"},
			host:       "",
			cookieName: anubis.CookieName,
		},
		{
			name:       "dynamic cookie domain",
			options:    Options{CookieDynamicDomain: true},
			host:       "techaro.lol",
			cookieName: anubis.CookieName,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := spawnAnubis(t, tt.options)
			rw := httptest.NewRecorder()

			srv.SetCookie(rw, CookieOpts{Value: "test", Host: tt.host})

			resp := rw.Result()
			cookies := resp.Cookies()

			ckie := cookies[0]

			if ckie.Name != tt.cookieName {
				t.Errorf("wanted cookie named %q, got cookie named %q", tt.cookieName, ckie.Name)
			}
		})
	}
}

func TestClearCookie(t *testing.T) {
	srv := spawnAnubis(t, Options{})
	rw := httptest.NewRecorder()

	srv.ClearCookie(rw, CookieOpts{Host: "localhost"})

	resp := rw.Result()

	cookies := resp.Cookies()

	if len(cookies) != 1 {
		t.Errorf("wanted 1 cookie, got %d cookies", len(cookies))
	}

	ckie := cookies[0]

	if ckie.Name != anubis.CookieName {
		t.Errorf("wanted cookie named %q, got cookie named %q", anubis.CookieName, ckie.Name)
	}

	if ckie.MaxAge != -1 {
		t.Errorf("wanted cookie max age of -1, got: %d", ckie.MaxAge)
	}
}

func TestClearCookieWithDomain(t *testing.T) {
	srv := spawnAnubis(t, Options{CookieDomain: "techaro.lol"})
	rw := httptest.NewRecorder()

	srv.ClearCookie(rw, CookieOpts{Host: "localhost"})

	resp := rw.Result()

	cookies := resp.Cookies()

	if len(cookies) != 1 {
		t.Errorf("wanted 1 cookie, got %d cookies", len(cookies))
	}

	ckie := cookies[0]

	if ckie.Name != anubis.CookieName {
		t.Errorf("wanted cookie named %q, got cookie named %q", anubis.CookieName, ckie.Name)
	}

	if ckie.MaxAge != -1 {
		t.Errorf("wanted cookie max age of -1, got: %d", ckie.MaxAge)
	}
}

func TestClearCookieWithDynamicDomain(t *testing.T) {
	srv := spawnAnubis(t, Options{CookieDynamicDomain: true})
	rw := httptest.NewRecorder()

	srv.ClearCookie(rw, CookieOpts{Host: "subdomain.xeiaso.net"})

	resp := rw.Result()

	cookies := resp.Cookies()

	if len(cookies) != 1 {
		t.Errorf("wanted 1 cookie, got %d cookies", len(cookies))
	}

	ckie := cookies[0]

	if ckie.Name != anubis.CookieName {
		t.Errorf("wanted cookie named %q, got cookie named %q", anubis.CookieName, ckie.Name)
	}

	if ckie.Domain != "xeiaso.net" {
		t.Errorf("wanted cookie domain %q, got cookie domain %q", "xeiaso.net", ckie.Domain)
	}

	if ckie.MaxAge != -1 {
		t.Errorf("wanted cookie max age of -1, got: %d", ckie.MaxAge)
	}
}

func TestRenderIndexRedirect(t *testing.T) {
	s := &Server{
		opts: Options{
			PublicUrl: "https://anubis.example.com",
		},
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Uri", "/foo")

	rr := httptest.NewRecorder()
	s.RenderIndex(rr, req, policy.CheckResult{}, nil, true)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rr.Code)
	}
	location := rr.Header().Get("Location")
	parsedURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse location URL %q: %v", location, err)
	}

	scheme := "https"
	if parsedURL.Scheme != scheme {
		t.Errorf("expected scheme to be %q, got %q", scheme, parsedURL.Scheme)
	}

	host := "anubis.example.com"
	if parsedURL.Host != host {
		t.Errorf("expected url to be %q, got %q", host, parsedURL.Host)
	}

	redir := parsedURL.Query().Get("redir")
	expectedRedir := "https://example.com/foo"
	if redir != expectedRedir {
		t.Errorf("expected redir param to be %q, got %q", expectedRedir, redir)
	}
}

func TestRenderIndexUnauthorized(t *testing.T) {
	s := &Server{
		opts: Options{
			PublicUrl: "",
		},
	}
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	s.RenderIndex(rr, req, policy.CheckResult{}, nil, true)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if body := rr.Body.String(); body != "Authorization required" {
		t.Errorf("expected body %q, got %q", "Authorization required", body)
	}
}

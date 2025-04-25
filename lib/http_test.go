package lib

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/TecharoHQ/anubis"
)

func TestClearCookie(t *testing.T) {
	srv := spawnAnubis(t, Options{})
	rw := httptest.NewRecorder()

	srv.ClearCookie(rw, srv.cookieName, "/")

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

	srv.ClearCookie(rw, srv.cookieName, "/")

	resp := rw.Result()

	cookies := resp.Cookies()

	if len(cookies) != 1 {
		t.Errorf("wanted 1 cookie, got %d cookies", len(cookies))
	}

	ckie := cookies[0]

	if ckie.Name != srv.cookieName {
		t.Errorf("wanted cookie named %q, got cookie named %q", srv.cookieName, ckie.Name)
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
	s.RenderIndex(rr, req, nil, true)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected status %d, got %d", http.StatusTemporaryRedirect, rr.Code)
	}
	location := rr.Header().Get("Location")
	parsedURL, _ := url.Parse(location)

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

	s.RenderIndex(rr, req, nil, true)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if body := rr.Body.String(); body != "Authorization required" {
		t.Errorf("expected body %q, got %q", "Authorization required", body)
	}
}

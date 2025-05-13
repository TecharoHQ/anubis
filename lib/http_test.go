package lib

import (
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis"
)

func TestClearCookie(t *testing.T) {
	srv := spawnAnubis(t, Options{})
	rw := httptest.NewRecorder()

	srv.ClearCookie(rw)

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

	srv.ClearCookie(rw)

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

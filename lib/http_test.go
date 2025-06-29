package lib

import (
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis"
)

func TestSetCookie(t *testing.T) {
	for _, tt := range []struct {
		name       string
		options    Options
		host       string
		cookieName string
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

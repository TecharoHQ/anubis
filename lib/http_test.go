package lib

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis"
)

func TestClearCookie(t *testing.T) {
	srv := spawnAnubis(t, Options{})
	rw := httptest.NewRecorder()

	srv.ClearCookie(rw, srv.cookieName, "/", "")

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

	srv.ClearCookie(rw, srv.cookieName, "/", srv.opts.CookieDomain)

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

func TestGetCookieScope(t *testing.T) {
	for _, tt := range []struct {
		hostHeader               string
		cookieDomain, cookieName string
	}{
		{
			hostHeader:   "xeiaso.net",
			cookieDomain: "xeiaso.net",
			cookieName:   "techaro.lol-anubis-auth",
		},
		{
			hostHeader:   "blog.xeiaso.net",
			cookieDomain: "xeiaso.net",
			cookieName:   "techaro.lol-anubis-auth",
		},
		{
			hostHeader:   "foo.shark-harmonic.ts.net",
			cookieDomain: "shark-harmonic.ts.net",
			cookieName:   "techaro.lol-anubis-auth",
		},
	} {
		t.Run(fmt.Sprint(tt.hostHeader, tt.cookieDomain, tt.cookieName), func(t *testing.T) {
			srv := spawnAnubis(t, Options{
				CookieDomain: "DYNAMIC_SECOND_LEVEL_DOMAIN",
			})
			cookieDomain, cookieName := srv.GetCookieScope(tt.hostHeader)

			if cookieDomain != tt.cookieDomain {
				t.Errorf("wanted cookie domain %q but got: %q", tt.cookieDomain, cookieDomain)
			}
			if cookieName != tt.cookieName {
				t.Errorf("wanted cookie name %q but got: %q", tt.cookieName, cookieName)
			}
		})
	}
}

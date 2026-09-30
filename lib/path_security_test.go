package lib

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis/lib/policy"
)

func TestAmbiguousPathCannotBypassDeny(t *testing.T) {
	pol, err := policy.ParseConfig(t.Context(), strings.NewReader(`bots:
- name: admin
  action: DENY
  path_regex: ^/admin
- name: public
  action: ALLOW
  path_regex: .*
`), "test.yaml", 0, "error", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/foo/%2e%2e/admin/secret", "//admin/secret", "/admin/./secret"} {
		forwarded := false
		srv := spawnAnubis(t, Options{Policy: pol, Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true })})
		req := httptest.NewRequest("GET", "http://example.com"+p, nil)
		req.Header.Set("X-Real-IP", "127.0.0.1")
		w := httptest.NewRecorder()
		srv.maybeReverseProxyOrPage(w, req)
		if forwarded {
			t.Errorf("forwarded ambiguous path %q", p)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("cache headers for %q: %v", p, w.Header())
		}
	}
}

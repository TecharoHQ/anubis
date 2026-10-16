package policy

import (
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
)

func TestPolicyRejectsAmbiguousPaths(t *testing.T) {
	for _, p := range []string{"/foo/%2e%2e/admin/secret", "/admin/./secret", "/public//../admin", "/admin//./secret"} {
		r := httptest.NewRequest("GET", "http://example.com"+p, nil)
		cfg := &ParsedConfig{}
		if cfg.ValidateRequestPath(r) == nil {
			t.Errorf("accepted %q", p)
		}
	}
	for _, p := range []string{"/", "/admin/", "/admin/secret", "/wiki//bin", "//admin/secret", "/wiki///", "/admin%2F%2F/secret"} {
		if err := (&ParsedConfig{}).ValidateRequestPath(httptest.NewRequest("GET", "http://example.com"+p, nil)); err != nil {
			t.Errorf("rejected %q: %v", p, err)
		}
	}
	for _, header := range []string{"X-Original-Uri", "X-Forwarded-Uri"} {
		for _, p := range []string{"/public/../admin", "/public//%2e%2e/admin", "/admin//%2e/secret", "%invalid"} {
			r := httptest.NewRequest("GET", "http://example.com/api/check", nil)
			r.Header.Set(header, p)
			if (&ParsedConfig{SubrequestMode: true}).ValidateRequestPath(r) == nil {
				t.Errorf("accepted %s URI %q", header, p)
			}
		}
	}
}

func TestRepeatedSlashesInPathRules(t *testing.T) {
	for _, target := range []string{"//admin/secret", "/admin//secret", "/admin%2F%2F/secret"} {
		for _, header := range []string{"", "X-Original-Uri", "X-Forwarded-Uri"} {
			t.Run(target+header, func(t *testing.T) {
				r := httptest.NewRequest("GET", "http://example.com"+target, nil)
				if header != "" {
					r = httptest.NewRequest("GET", "http://example.com/api/check", nil)
					r.Header.Set(header, target+"?next=/public")
				}
				if err := (&ParsedConfig{SubrequestMode: header != ""}).ValidateRequestPath(r); err != nil {
					t.Fatalf("rejected repeated slashes: %v", err)
				}
				pathBefore, escapedBefore, uriBefore := r.URL.Path, r.URL.EscapedPath(), r.RequestURI
				checker, err := NewPathChecker(`^/admin/secret$`, header != "")
				if err != nil {
					t.Fatal(err)
				}
				if matched, err := checker.Check(r); err != nil || !matched {
					t.Errorf("regex match=%v err=%v", matched, err)
				}
				celChecker, err := NewCELChecker(&config.ExpressionOrList{Expression: `path == "/admin/secret"`}, newTestDNS(t), header != "")
				if err != nil {
					t.Fatal(err)
				}
				if matched, err := celChecker.Check(r); err != nil || !matched {
					t.Errorf("CEL match=%v err=%v", matched, err)
				}
				if r.URL.Path != pathBefore || r.URL.EscapedPath() != escapedBefore || r.RequestURI != uriBefore {
					t.Fatal("path evaluation changed the request")
				}
				if header != "" && r.Header.Get(header) != target+"?next=/public" {
					t.Fatal("path evaluation changed the forwarded URI")
				}
			})
		}
	}
}

package policy

import (
	"net/http/httptest"
	"testing"
)

func TestPolicyRejectsAmbiguousPaths(t *testing.T) {
	for _, p := range []string{"/foo/%2e%2e/admin/secret", "//admin/secret", "/admin/./secret"} {
		r := httptest.NewRequest("GET", "http://example.com"+p, nil)
		cfg := &ParsedConfig{}
		if cfg.ValidateRequestPath(r) == nil {
			t.Errorf("accepted %q", p)
		}
	}
	for _, p := range []string{"/", "/admin/", "/admin/secret"} {
		if err := (&ParsedConfig{}).ValidateRequestPath(httptest.NewRequest("GET", "http://example.com"+p, nil)); err != nil {
			t.Errorf("rejected %q: %v", p, err)
		}
	}
	for _, p := range []string{"/public/../admin", "//admin", "%invalid"} {
		r := httptest.NewRequest("GET", "http://example.com/api/check", nil)
		r.Header.Set("X-Original-Uri", p)
		if (&ParsedConfig{SubrequestMode: true}).ValidateRequestPath(r) == nil {
			t.Errorf("accepted original URI %q", p)
		}
	}
}

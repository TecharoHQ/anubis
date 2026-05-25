package policy

import (
	"net/http"
	"testing"

	"github.com/TecharoHQ/anubis/internal/dns"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

func newTestDNS(t *testing.T) *dns.Dns {
	t.Helper()

	ctx := t.Context()
	memStore := memory.New(ctx)
	cache := dns.NewDNSCache(300, 300, memStore)
	return dns.New(ctx, cache)
}

func TestCELChecker_MapIterationWrappers(t *testing.T) {
	cfg := &config.ExpressionOrList{
		Expression: `headers.exists(k, k == "Accept") && query.exists(k, k == "format")`,
	}

	checker, err := NewCELChecker(cfg, newTestDNS(t), false)
	if err != nil {
		t.Fatalf("creating CEL checker failed: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com/?format=json", nil)
	if err != nil {
		t.Fatalf("making request failed: %v", err)
	}
	req.Header.Set("Accept", "application/json")

	got, err := checker.Check(req)
	if err != nil {
		t.Fatalf("checking expression failed: %v", err)
	}
	if !got {
		t.Fatal("expected expression to evaluate true")
	}
}

func TestCELChecker_PathWithForwardedUri(t *testing.T) {
	tests := []struct {
		name           string
		expression     string
		xForwardedUri  string
		urlPath        string
		subRequestMode bool
		want           bool
	}{
		{
			name:           "path matches X-Forwarded-Uri in subrequest mode",
			expression:     `path.startsWith("/admin")`,
			xForwardedUri:  "/admin/secret",
			urlPath:        "/.within.website/x/cmd/anubis/api/check",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "path with query string",
			expression:     `path.startsWith("/api/secret")`,
			xForwardedUri:  "/api/secret?token=abc",
			urlPath:        "/.within.website/x/cmd/anubis/api/check",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "path falls back to url path when no header",
			expression:     `path == "/public/page"`,
			urlPath:        "/public/page",
			subRequestMode: true,
			want:           true,
		},
		{
			name:           "non-subrequest mode ignores X-Forwarded-Uri",
			expression:     `path.startsWith("/admin")`,
			xForwardedUri:  "/admin/secret",
			urlPath:        "/public/page",
			subRequestMode: false,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ExpressionOrList{
				Expression: tt.expression,
			}
			checker, err := NewCELChecker(cfg, newTestDNS(t), tt.subRequestMode)
			if err != nil {
				t.Fatalf("NewCELChecker() error: %v", err)
			}

			req, err := http.NewRequest(http.MethodGet, "http://example.com"+tt.urlPath, nil)
			if err != nil {
				t.Fatalf("http.NewRequest: %v", err)
			}

			if tt.xForwardedUri != "" {
				req.Header.Set("X-Forwarded-Uri", tt.xForwardedUri)
			}

			got, err := checker.Check(req)
			if err != nil {
				t.Fatalf("Check() error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Check() = %v, want %v (subRequestMode=%v, urlPath=%q, X-Forwarded-Uri=%q)",
					got, tt.want, tt.subRequestMode, tt.urlPath, tt.xForwardedUri)
			}
		})
	}
}

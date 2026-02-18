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

	checker, err := NewCELChecker(cfg, newTestDNS(t))
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

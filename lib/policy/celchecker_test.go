package policy

import (
	"github.com/TecharoHQ/anubis/lib/policy/checker"
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

	celChecker, err := NewCELChecker(cfg, newTestDNS(t))
	if err != nil {
		t.Fatalf("creating CEL celChecker failed: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com/?format=json", nil)
	if err != nil {
		t.Fatalf("making request failed: %v", err)
	}
	req.Header.Set("Accept", "application/json")

	meta, err := checker.MetadataFromRequest(req)
	if err != nil {
		t.Fatalf("creating metadata from request failed: %v", err)
	}

	got, err := celChecker.Check(meta)
	if err != nil {
		t.Fatalf("checking expression failed: %v", err)
	}
	if !got {
		t.Fatal("expected expression to evaluate true")
	}
}

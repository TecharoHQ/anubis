package lib

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis/internal"
)

func TestPassChallengeRedirectIsUncacheable(t *testing.T) {
	srv := spawnAnubis(t, Options{Next: http.NewServeMux(), Policy: loadPolicies(t, "testdata/zero_difficulty.yaml", 0)})
	ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
	t.Cleanup(ts.Close)
	cli := httpClient(t)
	c := makeChallenge(t, ts, cli)
	resp := handleChallengeZeroDifficulty(t, ts, cli, c)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound || authCookie(srv, resp) == nil {
		t.Fatalf("challenge unsuccessful: %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Cache-Control"), "no-store") {
		t.Error("cookie redirect permits storage")
	}
}

package policy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/data"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/thoth/thothmock"
)

func TestDefaultPolicyMustParse(t *testing.T) {
	ctx := thothmock.WithMockThoth(t)

	fin, err := data.BotPolicies.Open("botPolicies.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close() //nolint:errcheck

	if _, err := ParseConfig(ctx, fin, "botPolicies.yaml", anubis.DefaultDifficulty, "info", false); err != nil {
		t.Fatalf("can't parse config: %v", err)
	}
}

func TestGoodConfigs(t *testing.T) {

	finfos, err := os.ReadDir("../config/testdata/good")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		t.Run(st.Name(), func(t *testing.T) {
			t.Run("with-thoth", func(t *testing.T) {
				fin, err := os.Open(filepath.Join("..", "config", "testdata", "good", st.Name()))
				if err != nil {
					t.Fatal(err)
				}
				defer fin.Close() //nolint:errcheck

				ctx := thothmock.WithMockThoth(t)
				if _, err := ParseConfig(ctx, fin, fin.Name(), anubis.DefaultDifficulty, "info", false); err != nil {
					t.Fatal(err)
				}
			})

			t.Run("without-thoth", func(t *testing.T) {
				fin, err := os.Open(filepath.Join("..", "config", "testdata", "good", st.Name()))
				if err != nil {
					t.Fatal(err)
				}
				defer fin.Close() //nolint:errcheck

				if _, err := ParseConfig(t.Context(), fin, fin.Name(), anubis.DefaultDifficulty, "info", false); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

func TestBadConfigs(t *testing.T) {
	ctx := thothmock.WithMockThoth(t)

	finfos, err := os.ReadDir("../config/testdata/bad")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		t.Run(st.Name(), func(t *testing.T) {
			fin, err := os.Open(filepath.Join("..", "config", "testdata", "bad", st.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer fin.Close() //nolint:errcheck

			if _, err := ParseConfig(ctx, fin, fin.Name(), anubis.DefaultDifficulty, "info", false); err == nil {
				t.Fatal(err)
			} else {
				t.Log(err)
			}
		})
	}
}

func TestPathCheckerStripsForwardedURIQuery(t *testing.T) {
	checker, err := NewPathChecker("^/admin$", true)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "https://anubis.local/.within.website/x/cmd/anubis/api/check", nil)
	req.Header.Set("X-Forwarded-Uri", "/admin?x=1")
	matched, err := checker.Check(req)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatalf("expected exact path checker to match forwarded URI when query string is appended")
	}
	req.Header.Set("X-Forwarded-Uri", "/admin")
	matched, err = checker.Check(req)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatalf("expected exact path checker to match forwarded URI without query string")
	}
}

func TestConfigReferencesJA4H(t *testing.T) {
	for _, tt := range []struct {
		name string
		bots []config.BotConfig
		want bool
	}{
		{
			name: "no bots",
			bots: nil,
			want: false,
		},
		{
			name: "unrelated rules",
			bots: []config.BotConfig{
				{Name: "ua", HeadersRegex: map[string]string{"User-Agent": "curl"}},
				{Name: "expr", Expression: &config.ExpressionOrList{Expression: `userAgent.contains("bot")`}},
			},
			want: false,
		},
		{
			name: "headers_regex exact match",
			bots: []config.BotConfig{
				{Name: "ja4h", HeadersRegex: map[string]string{internal.JA4HHeaderName: "t13d.*"}},
			},
			want: true,
		},
		{
			name: "headers_regex case-insensitive match",
			bots: []config.BotConfig{
				{Name: "ja4h", HeadersRegex: map[string]string{"x-http-fingerprint-ja4h": ".*"}},
			},
			want: true,
		},
		{
			name: "expression references header",
			bots: []config.BotConfig{
				{Name: "ja4h", Expression: &config.ExpressionOrList{Expression: `headers["X-Http-Fingerprint-Ja4h"] == "t13d"`}},
			},
			want: true,
		},
		{
			name: "expression list references header",
			bots: []config.BotConfig{
				{Name: "ja4h", Expression: &config.ExpressionOrList{Any: []string{
					`userAgent.contains("bot")`,
					`headers["X-Http-Fingerprint-Ja4h"] == "t13d"`,
				}}},
			},
			want: true,
		},
		{
			name: "expression missingHeader references header",
			bots: []config.BotConfig{
				{Name: "ja4h", Expression: &config.ExpressionOrList{Expression: `!missingHeader(headers, "X-Http-Fingerprint-Ja4h")`}},
			},
			want: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := configReferencesJA4H(tt.bots); got != tt.want {
				t.Errorf("configReferencesJA4H() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseConfigRemoteAddressesURL(t *testing.T) {
	body := `{
		"creationTime": "2025-01-02T03:04:05.000000",
		"prefixes": [{"ipv4Prefix": "20.42.10.176/28"}]
	}`

	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(ts.Close)

	yaml := fmt.Sprintf(`
bots:
  - name: openai-searchbot
    action: ALLOW
    remote_addresses_url: %q
`, ts.URL)

	type parseResult struct {
		cfg *ParsedConfig
		err error
	}
	ch := make(chan parseResult, 1)
	go func() {
		cfg, err := ParseConfig(t.Context(), strings.NewReader(yaml), "dynamic.yaml", anubis.DefaultDifficulty, "error", false)
		ch <- parseResult{cfg, err}
	}()

	var parsed *ParsedConfig
	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("ParseConfig: %v", res.err)
		}
		parsed = res.cfg
	case <-time.After(2 * time.Second):
		t.Fatal("ParseConfig blocked on remote fetch")
	}

	if len(parsed.Bots) != 1 {
		t.Fatalf("got %d bots, want 1", len(parsed.Bots))
	}

	close(release)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "20.42.10.176")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := parsed.Bots[0].Rules.Check(req)
		if err != nil {
			t.Fatalf("Check: %v", err)
		}
		if ok {
			miss := httptest.NewRequest(http.MethodGet, "/", nil)
			miss.Header.Set("X-Real-IP", "1.1.1.1")
			matched, err := parsed.Bots[0].Rules.Check(miss)
			if err != nil {
				t.Fatalf("Check unrelated IP: %v", err)
			}
			if matched {
				t.Fatal("unrelated IP should not match")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("remote addresses url list was not loaded in time")
}

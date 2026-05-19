package policy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/data"
	"github.com/TecharoHQ/anubis/lib/thoth/thothmock"
)

func TestDefaultPolicyMustParse(t *testing.T) {
	ctx := thothmock.WithMockThoth(t)

	fin, err := data.BotPolicies.Open("botPolicies.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close()

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
				defer fin.Close()

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
				defer fin.Close()

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
			defer fin.Close()

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

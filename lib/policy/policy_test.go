package policy

import (
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

	if _, err := ParseConfig(ctx, fin, "botPolicies.yaml", anubis.DefaultDifficulty, "info"); err != nil {
		t.Fatalf("can't parse config: %v", err)
	}
}

func TestGoodConfigs(t *testing.T) {

	finfos, err := os.ReadDir("../config/testdata/good")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			t.Run("with-thoth", func(t *testing.T) {
				fin, err := os.Open(filepath.Join("..", "config", "testdata", "good", st.Name()))
				if err != nil {
					t.Fatal(err)
				}
				defer fin.Close()

				ctx := thothmock.WithMockThoth(t)
				if _, err := ParseConfig(ctx, fin, fin.Name(), anubis.DefaultDifficulty, "info"); err != nil {
					t.Fatal(err)
				}
			})

			t.Run("without-thoth", func(t *testing.T) {
				fin, err := os.Open(filepath.Join("..", "config", "testdata", "good", st.Name()))
				if err != nil {
					t.Fatal(err)
				}
				defer fin.Close()

				if _, err := ParseConfig(t.Context(), fin, fin.Name(), anubis.DefaultDifficulty, "info"); err != nil {
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
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			fin, err := os.Open(filepath.Join("..", "config", "testdata", "bad", st.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer fin.Close()

			if _, err := ParseConfig(ctx, fin, fin.Name(), anubis.DefaultDifficulty, "info"); err == nil {
				t.Fatal(err)
			} else {
				t.Log(err)
			}
		})
	}
}

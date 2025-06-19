package lib

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal/thoth/thothmock"
	"github.com/TecharoHQ/anubis/lib/policy"
)

func TestInvalidChallengeMethod(t *testing.T) {
	if _, err := LoadPoliciesOrDefault(t.Context(), "testdata/invalid-challenge-method.yaml", 4); !errors.Is(err, policy.ErrChallengeRuleHasWrongAlgorithm) {
		t.Fatalf("wanted error %v but got %v", policy.ErrChallengeRuleHasWrongAlgorithm, err)
	}
}

func TestBadConfigs(t *testing.T) {
	finfos, err := os.ReadDir("policy/config/testdata/bad")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			if _, err := LoadPoliciesOrDefault(t.Context(), filepath.Join("policy", "config", "testdata", "bad", st.Name()), anubis.DefaultDifficulty); err == nil {
				t.Fatal(err)
			} else {
				t.Log(err)
			}
		})
	}
}

func TestGoodConfigs(t *testing.T) {
	finfos, err := os.ReadDir("policy/config/testdata/good")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			t.Run("with-thoth", func(t *testing.T) {
				ctx := thothmock.WithMockThoth(t)
				if _, err := LoadPoliciesOrDefault(ctx, filepath.Join("policy", "config", "testdata", "good", st.Name()), anubis.DefaultDifficulty); err != nil {
					t.Fatal(err)
				}
			})

			t.Run("without-thoth", func(t *testing.T) {
				if _, err := LoadPoliciesOrDefault(t.Context(), filepath.Join("policy", "config", "testdata", "good", st.Name()), anubis.DefaultDifficulty); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

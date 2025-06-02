package lib

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/policy"
)

func TestInvalidChallengeMethod(t *testing.T) {
	if _, err := LoadPoliciesOrDefault("testdata/invalid-challenge-method.yaml", 4); !errors.Is(err, policy.ErrChallengeRuleHasWrongAlgorithm) {
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
			if _, err := LoadPoliciesOrDefault(filepath.Join("policy", "config", "testdata", "good", st.Name()), anubis.DefaultDifficulty); err == nil {
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
			if _, err := LoadPoliciesOrDefault(filepath.Join("policy", "config", "testdata", "good", st.Name()), anubis.DefaultDifficulty); err != nil {
				t.Fatal(err)
			}
		})
	}
}

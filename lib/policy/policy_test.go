package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/data"
)

func TestDefaultPolicyMustParse(t *testing.T) {
	fin, err := data.BotPolicies.Open("botPolicies.json")
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close()

	if _, err := ParseConfig(fin, "botPolicies.json", anubis.DefaultDifficulty); err != nil {
		t.Fatalf("can't parse config: %v", err)
	}
}

func TestGoodConfigs(t *testing.T) {
	finfos, err := os.ReadDir("config/testdata/good")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			fin, err := os.Open(filepath.Join("config", "testdata", "good", st.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer fin.Close()

			if _, err := ParseConfig(fin, fin.Name(), anubis.DefaultDifficulty); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBadConfigs(t *testing.T) {
	finfos, err := os.ReadDir("config/testdata/bad")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			fin, err := os.Open(filepath.Join("config", "testdata", "bad", st.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer fin.Close()

			if _, err := ParseConfig(fin, fin.Name(), anubis.DefaultDifficulty); err == nil {
				t.Fatal(err)
			} else {
				t.Log(err)
			}
		})
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPolicyMustParse(t *testing.T) {
	fin, err := static.Open("botPolicies.json")
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close()

	if _, err := parseConfig(fin, "botPolicies.json", defaultDifficulty); err != nil {
		t.Fatalf("can't parse config: %v", err)
	}
}

func TestGoodConfigs(t *testing.T) {
	finfos, err := os.ReadDir("internal/config/testdata/good")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			fin, err := os.Open(filepath.Join("internal", "config", "testdata", "good", st.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer fin.Close()

			if _, err := parseConfig(fin, fin.Name(), defaultDifficulty); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBadConfigs(t *testing.T) {
	finfos, err := os.ReadDir("internal/config/testdata/bad")
	if err != nil {
		t.Fatal(err)
	}

	for _, st := range finfos {
		st := st
		t.Run(st.Name(), func(t *testing.T) {
			fin, err := os.Open(filepath.Join("internal", "config", "testdata", "bad", st.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer fin.Close()

			if _, err := parseConfig(fin, fin.Name(), defaultDifficulty); err == nil {
				t.Fatal(err)
			} else {
				t.Log(err)
			}
		})
	}
}

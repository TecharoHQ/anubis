package path

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFactoryGood(t *testing.T) {
	files, err := os.ReadDir("./testdata/good")
	if err != nil {
		t.Fatal(err)
	}

	fac := Factory{}

	for _, fname := range files {
		t.Run(fname.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "good", fname.Name()))
			if err != nil {
				t.Fatal(err)
			}

			if err := fac.Valid(t.Context(), json.RawMessage(data)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFactoryBad(t *testing.T) {
	files, err := os.ReadDir("./testdata/bad")
	if err != nil {
		t.Fatal(err)
	}

	fac := Factory{}

	for _, fname := range files {
		t.Run(fname.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "bad", fname.Name()))
			if err != nil {
				t.Fatal(err)
			}

			if err := fac.Valid(t.Context(), json.RawMessage(data)); err == nil {
				t.Fatal("expected validation to fail")
			}
		})
	}
}

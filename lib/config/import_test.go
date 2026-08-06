package config

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"github.com/TecharoHQ/anubis/data"
)

func TestGlobMatch(t *testing.T) {
	for _, tt := range []struct {
		globbedPath string
		matches     []string
		err         error
	}{
		{
			globbedPath: "(data)/apps/allow-api-routes*.yaml",
			matches:     []string{"apps/allow-api-routes.yaml"},
		},
		{
			globbedPath: "./testdata/hack-test.*",
			matches:     []string{"testdata/hack-test.json", "testdata/hack-test.yaml"},
		},
		{
			globbedPath: "/does/not/exist/*",
			err:         ErrGlobMatchedNothing,
		},
		{
			// filepath.Match has no alternation syntax, so the braces are
			// matched literally and nothing is found.
			globbedPath: "(data)/bots/*.{yaml,yml}",
			err:         ErrGlobMatchedNothing,
		},
	} {
		t.Run(tt.globbedPath, func(t *testing.T) {
			_, matches, err := globMatch(tt.globbedPath)
			if !errors.Is(err, tt.err) {
				t.Logf("wanted error: %v", tt.err)
				t.Logf("   got error: %v", err)
				t.Error("unexpected error received")
			}

			if !slices.Equal(tt.matches, matches) {
				t.Logf("wanted files: %#v", tt.matches)
				t.Logf("   got files: %#v", matches)
				t.Error("unexpected file matches")
			}
		})
	}
}

func TestImportMultiDocument(t *testing.T) {
	for _, tt := range []struct {
		name        string
		globbedPath string
		botCount    int
	}{
		{"multi-document", "./testdata/multi-document/*.yaml", 3},
		{"single-file", "./testdata/multi-document/small-internet-browsers.yaml", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			is := &ImportStatement{Import: tt.globbedPath}

			if err := is.Valid(); err != nil {
				t.Fatal(err)
			}

			if len(is.Bots) != tt.botCount {
				t.Errorf("wanted bot count: %d, got: %d", tt.botCount, len(is.Bots))
			}
		})
	}
}

// TestImportStatementGlobMatchedNothing asserts that an import statement whose
// glob pattern matches no files fails with ErrGlobMatchedNothing instead of the
// io.EOF that the YAML decoder would return for an empty document.
func TestImportStatementGlobMatchedNothing(t *testing.T) {
	for _, globbedPath := range []string{
		"(data)/bots/*.yml",
		"(data)/does-not-exist/*.yaml",
		"./testdata/does-not-exist/*.yaml",
	} {
		t.Run(globbedPath, func(t *testing.T) {
			is := &ImportStatement{Import: globbedPath}

			err := is.Valid()
			if !errors.Is(err, ErrGlobMatchedNothing) {
				t.Logf("wanted error: %v", ErrGlobMatchedNothing)
				t.Logf("   got error: %v", err)
				t.Error("unexpected error received")
			}
		})
	}
}

// TestImportStatement asserts that every policy snippet in the data folder can
// be imported and is valid. The list of files is discovered by walking the
// embedded filesystem so that new folders and new snippets are covered without
// having to remember to add them here.
func TestImportStatement(t *testing.T) {
	var importPaths []string

	if err := fs.WalkDir(data.BotPolicies, ".", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		case path == "botPolicies.yaml": // an entire config, not a list of bots
			return nil
		}

		switch filepath.Ext(path) {
		case ".yaml", ".yml":
			importPaths = append(importPaths, "(data)/"+path)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if len(importPaths) == 0 {
		t.Fatal("no policy snippets found in the embedded data folder")
	}

	for _, importPath := range importPaths {
		t.Run(importPath, func(t *testing.T) {
			is := &ImportStatement{
				Import: importPath,
			}

			if err := is.Valid(); err != nil {
				t.Errorf("validation error: %v", err)
			}

			if len(is.Bots) == 0 {
				t.Error("wanted bot definitions, but got none")
			}
		})
	}
}

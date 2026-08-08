package data

import (
	"io/fs"
	"os"
	"path"
	"testing"

	"k8s.io/apimachinery/pkg/util/yaml"
)

// TestBotPoliciesEmbed ensures all YAML files in the directory tree
// are accessible in the embedded BotPolicies filesystem.
func TestBotPoliciesEmbed(t *testing.T) {
	// Walk the tree instead of globbing. filepath.Glob does not treat ** as
	// a recursive wildcard, so "./**/*.yaml" only ever matched files exactly
	// one directory deep, silently skipping botPolicies.yaml and everything
	// under directories like clients/small-internet-browsers.
	var yamlFiles []string
	if err := fs.WalkDir(os.DirFS("."), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(p) != ".yaml" {
			return nil
		}
		yamlFiles = append(yamlFiles, p)
		return nil
	}); err != nil {
		t.Fatalf("Failed to walk YAML files: %v", err)
	}

	if len(yamlFiles) == 0 {
		t.Fatal("No YAML files found in directory tree")
	}

	t.Logf("Found %d YAML files to verify", len(yamlFiles))

	for _, embeddedPath := range yamlFiles {
		t.Run(embeddedPath, func(t *testing.T) {
			content, err := BotPolicies.ReadFile(embeddedPath)
			if err != nil {
				t.Errorf("Failed to read %s from embedded filesystem: %v", embeddedPath, err)
				return
			}

			if len(content) == 0 {
				t.Errorf("File %s exists in embedded filesystem but is empty", embeddedPath)
			}
		})

		t.Run("verify-yaml/"+embeddedPath, func(t *testing.T) {
			fin, err := BotPolicies.Open(embeddedPath)
			if err != nil {
				t.Errorf("Failed to read %s from embedded filesystem: %v", embeddedPath, err)
				return
			}
			//nolint:errcheck
			defer fin.Close()

			var result any
			if err := yaml.NewYAMLToJSONDecoder(fin).Decode(&result); err != nil {
				t.Errorf("can't parse as YAML: %v", err)
			}
		})
	}
}

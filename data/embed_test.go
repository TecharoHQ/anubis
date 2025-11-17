package data

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestBotPoliciesEmbed ensures all YAML files in the directory tree
// are accessible in the embedded BotPolicies filesystem.
func TestBotPoliciesEmbed(t *testing.T) {
	yamlFiles, err := filepath.Glob("./**/*.yaml")
	if err != nil {
		t.Fatalf("Failed to glob YAML files: %v", err)
	}

	if len(yamlFiles) == 0 {
		t.Fatal("No YAML files found in directory tree")
	}

	t.Logf("Found %d YAML files to verify", len(yamlFiles))

	for _, filePath := range yamlFiles {
		embeddedPath := strings.TrimPrefix(filePath, "./")

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
	}
}

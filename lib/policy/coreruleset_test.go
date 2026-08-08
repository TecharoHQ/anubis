package policy

import (
	"net/http"
	"testing"

	"github.com/TecharoHQ/anubis/data"
	"github.com/TecharoHQ/anubis/lib/config"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// loadCoreruleset reads a generated Core Rule Set snippet out of the embedded
// data folder and returns its rules keyed by name.
func loadCoreruleset(t *testing.T, fname string) map[string]config.BotConfig {
	t.Helper()

	fin, err := data.BotPolicies.Open(fname)
	if err != nil {
		t.Fatalf("can't open %s: %v", fname, err)
	}
	t.Cleanup(func() { _ = fin.Close() })

	var bots []config.BotConfig
	if err := yaml.NewYAMLToJSONDecoder(fin).Decode(&bots); err != nil {
		t.Fatalf("can't parse %s: %v", fname, err)
	}

	result := make(map[string]config.BotConfig, len(bots))
	for _, b := range bots {
		if err := b.Valid(); err != nil {
			t.Errorf("rule %q is not valid: %v", b.Name, err)
		}

		// Every rule ported from a severity:'CRITICAL' upstream rule adds
		// tx.critical_anomaly_score, which is 5.
		if b.Action != config.RuleWeigh {
			t.Errorf("rule %q: got action %s, want %s", b.Name, b.Action, config.RuleWeigh)
		}
		if b.Weight == nil || b.Weight.Adjust != 5 {
			t.Errorf("rule %q: got weight %v, want 5", b.Name, b.Weight)
		}

		result[b.Name] = b
	}

	return result
}

// TestCoreruleset930140 pins the behaviour of the rule generated from
// crs:930140, which runs @pmFromFile over REQUEST_FILENAME. @pmFromFile is a
// case-insensitive substring match, so an entry has to match anywhere in the
// path and regardless of case.
func TestCoreruleset930140(t *testing.T) {
	rules := loadCoreruleset(t, "security/coreruleset/00-ai-agent-configs.yaml")

	rule, ok := rules["coreruleset-930140-ai-critical-artifacts"]
	if !ok {
		t.Fatal("rule coreruleset-930140-ai-critical-artifacts is missing")
	}

	checker, err := NewCELChecker(rule.Expression, newTestDNS(t), false)
	if err != nil {
		t.Fatalf("can't compile rule: %v", err)
	}

	for _, tt := range []struct {
		name string
		path string
		want bool
	}{
		{"artifact at the document root", "/.claude/settings.json", true},
		{"bare directory at the root", "/.cursor/", true},

		// @pmFromFile matches anywhere in the path, not only at the start.
		{"artifact one folder down", "/docs/.claude/settings.local.json", true},
		{"artifact several folders down", "/a/b/c/.aider/config.yml", true},
		{"artifact in a checked out repository", "/repo/.codex/config.toml", true},

		// @pmFromFile folds case.
		{"upper case directory", "/.CLAUDE/settings.json", true},
		{"mixed case, one folder down", "/Docs/.Cursor/mcp.json", true},

		// Entries without a trailing slash are plain substrings upstream.
		{"bare entry used as a directory", "/.crush/config", true},
		{"bare entry used as a file", "/.crush", true},
		{"bare entry one folder down", "/project/.qwen_code", true},

		{"ordinary page", "/index.html", false},
		{"ordinary API call", "/api/v1/users", false},
		{"site root", "/", false},
		{"folder that only looks similar", "/claude/index.html", false},
		{"file that only looks similar", "/my-cursor-theme.css", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "https://example.com"+tt.path, nil)
			if err != nil {
				t.Fatalf("can't make request: %v", err)
			}

			got, err := checker.Check(req)
			if err != nil {
				t.Fatalf("can't check request: %v", err)
			}

			if got != tt.want {
				t.Errorf("path %q: got %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

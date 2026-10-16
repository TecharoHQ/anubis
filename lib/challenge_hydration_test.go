package lib

import (
	"log/slog"
	"testing"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
)

func TestHydrateChallengeRulePreservesPolicy(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "defaults"}[configured], func(t *testing.T) {
			srv := &Server{policy: &policy.ParsedConfig{DefaultDifficulty: 4}, logger: slog.Default()}
			rule := &policy.Bot{}
			if configured {
				rule.Challenge = &config.ChallengeRules{}
			}
			issued := &challenge.Challenge{Method: "fast", Difficulty: 3}
			hydrated := srv.hydrateChallengeRule(rule, issued, slog.Default())
			if hydrated.Challenge.Difficulty != 3 || hydrated.Challenge.Algorithm != "fast" {
				t.Fatal("stored challenge settings were not hydrated")
			}
			if configured {
				if rule.Challenge.Difficulty != 0 || rule.Challenge.Algorithm != "" {
					t.Fatal("hydration modified shared policy settings")
				}
			} else if rule.Challenge != nil {
				t.Fatal("hydration modified shared policy rule")
			}
		})
	}
}

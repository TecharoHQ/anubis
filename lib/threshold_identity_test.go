package lib

import (
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
)

func TestThresholdPolicyHashesAreDistinct(t *testing.T) {
	srv := spawnAnubis(t, Options{Policy: loadPolicies(t, "testdata/zero_difficulty.yaml", 0)})
	srv.policy.Bots = nil
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "192.0.2.1")
	hashes := make(map[string]string)
	for _, name := range []string{"easy", "hard", "default", "harder", "expression"} {
		srv.policy.Thresholds = nil
		if name != "default" {
			thresholdName, expression, difficulty := name, "true", 1
			if name == "harder" {
				thresholdName = "easy"
				difficulty = 2
			}
			if name == "expression" {
				thresholdName = "easy"
				expression = "weight >= 0"
			}
			threshold, err := policy.ParsedThresholdFromConfig(config.Threshold{Name: thresholdName, Expression: &config.ExpressionOrList{Expression: expression}, Action: config.RuleChallenge, Challenge: &config.ChallengeRules{Algorithm: "fast", Difficulty: difficulty}})
			if err != nil {
				t.Fatal(err)
			}
			srv.policy.Thresholds = []*policy.Threshold{threshold}
		}
		_, bot, err := srv.check(req, slog.Default())
		if err != nil {
			t.Fatal(err)
		}
		if other, ok := hashes[bot.Hash()]; ok {
			t.Errorf("%s shares policy hash with %s", name, other)
		}
		hashes[bot.Hash()] = name
	}
}

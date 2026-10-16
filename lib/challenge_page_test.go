package lib

import (
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/challenge/challengetest"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
)

// TestChallengePagesHaveScriptWatchdog makes sure every challenge that hands
// its work to main.mjs also ships the bootstrap watchdog that re-injects
// main.mjs when it fails to load.
func TestChallengePagesHaveScriptWatchdog(t *testing.T) {
	needsWatchdog := map[string]bool{
		"argon2id":    true,
		"fast":        true,
		"hashx":       true,
		"metarefresh": false,
		"preact":      false,
		"sha256":      true,
		"slow":        true,
	}

	methods := challenge.Methods()

	for _, method := range methods {
		if _, ok := needsWatchdog[method]; !ok {
			t.Errorf("challenge method %s is not listed in this test: does its page need the bootstrap watchdog?", method)
		}
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			impl, ok := challenge.Get(method)
			if !ok {
				t.Fatalf("challenge method %s is registered but can't be fetched", method)
			}

			in := &challenge.IssueInput{
				Rule: &policy.Bot{
					Challenge: &config.ChallengeRules{
						Algorithm:  method,
						Difficulty: 4,
					},
				},
				Challenge: challengetest.New(t),
			}

			cmp, err := impl.Issue(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil), slog.With(), in)
			if err != nil {
				t.Fatalf("can't issue challenge: %v", err)
			}

			var sb strings.Builder
			if err := cmp.Render(t.Context(), &sb); err != nil {
				t.Fatalf("can't render challenge page: %v", err)
			}

			out := sb.String()
			markers := []string{`id="anubis-main"`, `id="anubis-script-error"`, `id="progress"`}

			for _, marker := range markers {
				got := strings.Contains(out, marker)
				if want := needsWatchdog[method]; got != want {
					t.Errorf("page contains %s: got %v, wanted %v", marker, got, want)
				}
			}
		})
	}
}

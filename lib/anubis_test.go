package lib

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis"
)

func spawnAnubis(t *testing.T, h http.Handler) string {
	t.Helper()

	policy, err := LoadPoliciesOrDefault("", anubis.DefaultDifficulty)
	if err != nil {
		t.Fatal(err)
	}

	s, err := New(Options{
		Next:           h,
		Policy:         policy,
		ServeRobotsTXT: true,
	})
	if err != nil {
		t.Fatalf("can't construct libanubis.Server: %v", err)
	}

	ts := httptest.NewServer(s)
	t.Log(ts.URL)

	t.Cleanup(func() {
		ts.Close()
	})

	return ts.URL
}

func TestCheckDefaultDifficultyMatchesPolicy(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	for i := 1; i < 10; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			policy, err := LoadPoliciesOrDefault("", i)
			if err != nil {
				t.Fatal(err)
			}

			s, err := New(Options{
				Next:           h,
				Policy:         policy,
				ServeRobotsTXT: true,
			})
			if err != nil {
				t.Fatalf("can't construct libanubis.Server: %v", err)
			}

			req, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			req.Header.Add("X-Real-Ip", "127.0.0.1")

			_, bot, err := s.check(req)
			if err != nil {
				t.Fatal(err)
			}

			if bot.Challenge.Difficulty != i {
				t.Errorf("Challenge.Difficulty is wrong, wanted %d, got: %d", i, bot.Challenge.Difficulty)
			}

			if bot.Challenge.ReportAs != i {
				t.Errorf("Challenge.ReportAs is wrong, wanted %d, got: %d", i, bot.Challenge.ReportAs)
			}
		})
	}
}

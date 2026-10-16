package lib

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/store"
)

func TestPassChallengeRequiresIssuedPolicy(t *testing.T) {
	for _, change := range []string{"hash", "difficulty", "algorithm", "extensions", "action"} {
		t.Run(change, func(t *testing.T) {
			srv := spawnAnubis(t, Options{Next: http.NewServeMux(), Policy: loadPolicies(t, "./testdata/zero_difficulty.yaml", 0)})
			ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
			t.Cleanup(ts.Close)
			cli := httpClient(t)
			issued := makeChallenge(t, ts, cli)
			j := store.JSON[challenge.Challenge]{Underlying: srv.store}
			c, err := j.Get(context.Background(), "challenge:"+issued.ID)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "hash":
				c.PolicyRuleHash = "other-rule"
			case "difficulty":
				c.Difficulty = 1
			case "algorithm":
				c.Method = "slow"
			case "extensions":
				c.Extensions = []string{"css-load"}
			case "action":
				for _, threshold := range srv.policy.Thresholds {
					threshold.Action = "DENY"
				}
			}
			if err := j.Set(context.Background(), "challenge:"+c.ID, c, 30*time.Minute); err != nil {
				t.Fatal(err)
			}
			resp := handleChallengeZeroDifficulty(t, ts, cli, issued)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode < 400 || authCookie(srv, resp) != nil {
				t.Errorf("changed %s accepted: status %d", change, resp.StatusCode)
			}
		})
	}
}

func TestChallengeCannotTransferToAnotherRule(t *testing.T) {
	pol := loadPolicies(t, "testdata/rule_change.yaml", 0)
	srv := spawnAnubis(t, Options{Next: http.NewServeMux(), Policy: pol})
	req := httptest.NewRequest("GET", "/old", nil)
	req.Header.Set("X-Real-IP", "192.0.2.1")
	lg, req := srv.getRequestLogger(req)
	cr, rule, err := srv.check(req, lg)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := srv.issueChallenge(req.Context(), req, lg, cr, rule)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/old", "/new"} {
		pass := httptest.NewRequest("GET", "/?redir="+path+"&id="+issued.ID+"&nonce=0&elapsedTime=1&response="+internal.SHA256sum(issued.RandomData+"0"), nil)
		pass.Header.Set("X-Real-IP", "192.0.2.1")
		pass.AddCookie(&http.Cookie{Name: srv.cookieName(anubis.TestCookieName), Value: issued.ID})
		rec := httptest.NewRecorder()
		srv.PassChallenge(rec, pass)
		if path == "/old" {
			if rec.Code != http.StatusFound {
				t.Fatalf("same rule with omitted challenge config rejected: %d", rec.Code)
			}
			issued.Spent = false
			j := store.JSON[challenge.Challenge]{Underlying: srv.store}
			if err := j.Set(req.Context(), "challenge:"+issued.ID, *issued, time.Minute); err != nil {
				t.Fatal(err)
			}
		} else if rec.Code != http.StatusForbidden || authCookie(srv, rec.Result()) != nil {
			t.Fatalf("rule transfer accepted: %d", rec.Code)
		}
	}
}

package proofofwork

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
)

func mkRequest(t *testing.T, values map[string]string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	q := req.URL.Query()

	for k, v := range values {
		q.Set(k, v)
	}

	req.URL.RawQuery = q.Encode()

	return req
}

func TestBasic(t *testing.T) {
	i := &Impl{Algorithm: "fast"}
	bot := &policy.Bot{
		Challenge: &config.ChallengeRules{
			Algorithm:  "fast",
			Difficulty: 0,
			ReportAs:   0,
		},
	}
	const challengeStr = "hunter"
	const response = "2652bdba8fb4d2ab39ef28d8534d7694c557a4ae146c1e9237bd8d950280500e"

	for _, cs := range []struct {
		name         string
		req          *http.Request
		err          error
		challengeStr string
	}{
		{
			name: "allgood",
			req: mkRequest(t, map[string]string{
				"nonce":       "0",
				"elapsedTime": "69",
				"response":    response,
			}),
			err:          nil,
			challengeStr: challengeStr,
		},
		{
			name:         "no-params",
			req:          mkRequest(t, map[string]string{}),
			err:          challenge.ErrMissingField,
			challengeStr: challengeStr,
		},
		{
			name: "missing-nonce",
			req: mkRequest(t, map[string]string{
				"elapsedTime": "69",
				"response":    response,
			}),
			err:          challenge.ErrMissingField,
			challengeStr: challengeStr,
		},
		{
			name: "missing-elapsedTime",
			req: mkRequest(t, map[string]string{
				"nonce":    "0",
				"response": response,
			}),
			err:          challenge.ErrMissingField,
			challengeStr: challengeStr,
		},
		{
			name: "missing-response",
			req: mkRequest(t, map[string]string{
				"nonce":       "0",
				"elapsedTime": "69",
			}),
			err:          challenge.ErrMissingField,
			challengeStr: challengeStr,
		},
		{
			name: "wrong-nonce-format",
			req: mkRequest(t, map[string]string{
				"nonce":       "taco",
				"elapsedTime": "69",
				"response":    response,
			}),
			err:          challenge.ErrInvalidFormat,
			challengeStr: challengeStr,
		},
		{
			name: "wrong-elapsedTime-format",
			req: mkRequest(t, map[string]string{
				"nonce":       "0",
				"elapsedTime": "taco",
				"response":    response,
			}),
			err:          challenge.ErrInvalidFormat,
			challengeStr: challengeStr,
		},
		{
			name: "invalid-response",
			req: mkRequest(t, map[string]string{
				"nonce":       "0",
				"elapsedTime": "69",
				"response":    response,
			}),
			err:          challenge.ErrFailed,
			challengeStr: "Tacos are tasty",
		},
	} {
		t.Run(cs.name, func(t *testing.T) {
			lg := slog.With()

			i.Setup(http.NewServeMux())

			inp := &challenge.IssueInput{
				Rule: bot,
				Challenge: &challenge.Challenge{
					RandomData: cs.challengeStr,
				},
			}

			if _, err := i.Issue(httptest.NewRecorder(), cs.req, lg, inp); err != nil {
				t.Errorf("can't issue challenge: %v", err)
			}

			if err := i.Validate(cs.req, lg, &challenge.ValidateInput{
				Rule: bot,
				Challenge: &challenge.Challenge{
					RandomData: cs.challengeStr,
				},
			}); !errors.Is(err, cs.err) {
				t.Errorf("got wrong error from Validate, got %v but wanted %v", err, cs.err)
			}
		})
	}
}

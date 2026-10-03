package lib

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/challenge"
)

type failingChallenge struct {
	challenge.Impl
	err error
}

func (f failingChallenge) Validate(*http.Request, *slog.Logger, *challenge.ValidateInput) error {
	return f.err
}

func TestPassChallengeRejectsEveryValidationError(t *testing.T) {
	original, _ := challenge.Get("fast")
	t.Cleanup(func() { challenge.Register("fast", original) })
	for _, tt := range []struct {
		name string
		err  error
	}{
		{"plain", errors.New("validation unavailable")},
		{"wrapped without sentinel", challenge.NewError("validate", "internal WASM error", errors.New("validation unavailable"))},
		{"failed sentinel", challenge.NewError("validate", "invalid response", challenge.ErrFailed)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			challenge.Register("fast", failingChallenge{Impl: original, err: tt.err})
			srv := spawnAnubis(t, Options{Next: http.NewServeMux(), Policy: loadPolicies(t, "./testdata/zero_difficulty.yaml", 0)})
			ts := httptest.NewServer(internal.RemoteXRealIP(true, "tcp", srv))
			t.Cleanup(ts.Close)
			cli := httpClient(t)
			chall := makeChallenge(t, ts, cli)
			resp := handleChallengeZeroDifficulty(t, ts, cli, chall)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode < 400 {
				t.Errorf("validation error accepted: status %d", resp.StatusCode)
			}
			if authCookie(srv, resp) != nil {
				t.Error("validation error minted clearance cookie")
			}
		})
	}
}

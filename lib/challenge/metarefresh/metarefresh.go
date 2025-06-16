package metarefresh

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/web"
	"github.com/a-h/templ"
)

//go:generate go tool github.com/a-h/templ/cmd/templ generate

func init() {
	challenge.Register("metarefresh", &Impl{})
}

type Impl struct{}

func (i *Impl) Setup(mux *http.ServeMux) {}

func (i *Impl) Issue(r *http.Request, lg *slog.Logger, rule *policy.Bot, challenge string, ogTags map[string]string) (templ.Component, error) {
	u, err := r.URL.Parse(anubis.BasePrefix + "/.within.website/x/cmd/anubis/api/pass-challenge")
	if err != nil {
		return nil, fmt.Errorf("can't render page: %w", err)
	}

	q := u.Query()
	q.Set("redir", r.URL.String())
	q.Set("challenge", challenge)
	u.RawQuery = q.Encode()

	component, err := web.BaseWithChallengeAndOGTags("Making sure you're not a bot!", page(challenge, u.String(), rule.Challenge.Difficulty), challenge, rule.Challenge, ogTags)
	if err != nil {
		return nil, fmt.Errorf("can't render page: %w", err)
	}

	return component, nil
}

func (i *Impl) Validate(r *http.Request, lg *slog.Logger, rule *policy.Bot, wantChallenge string) error {
	gotChallenge := r.FormValue("challenge")

	if subtle.ConstantTimeCompare([]byte(wantChallenge), []byte(gotChallenge)) != 1 {
		return challenge.NewError("validate", "invalid response", fmt.Errorf("%w: wanted response %s but got %s", challenge.ErrFailed, wantChallenge, gotChallenge))
	}

	return nil
}

package metarefresh

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/localization"
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

func (i *Impl) Issue(r *http.Request, lg *slog.Logger, in *challenge.IssueInput) (templ.Component, error) {
	u, err := r.URL.Parse(anubis.BasePrefix + "/.within.website/x/cmd/anubis/api/pass-challenge")
	if err != nil {
		return nil, fmt.Errorf("can't render page: %w", err)
	}

	q := u.Query()
	q.Set("redir", r.URL.String())
	q.Set("challenge", in.Challenge)
	u.RawQuery = q.Encode()

	loc := localization.GetLocalizer(r)
	component, err := web.BaseWithChallengeAndOGTags(loc.T("making_sure_not_bot"), page(in.Challenge, u.String(), in.Rule.Challenge.Difficulty, loc), in.Impressum, in.Challenge, in.Rule.Challenge, in.OGTags, loc)

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

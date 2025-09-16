package metarefresh

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/a-h/templ"
)

//go:generate go tool github.com/a-h/templ/cmd/templ generate

func init() {
	challenge.Register("metarefresh", &Impl{})
}

type Impl struct{}

func (i *Impl) Setup(mux *http.ServeMux) {}

func (i *Impl) Issue(w http.ResponseWriter, r *http.Request, lg *slog.Logger, in *challenge.IssueInput) (templ.Component, error) {
	u, err := r.URL.Parse(anubis.BasePrefix + "/.within.website/x/cmd/anubis/api/pass-challenge")
	if err != nil {
		return nil, fmt.Errorf("can't render page: %w", err)
	}

	q := u.Query()
	q.Set("redir", r.URL.String())
	q.Set("challenge", in.Challenge.RandomData)
	q.Set("id", in.Challenge.ID)
	u.RawQuery = q.Encode()

	showMeta := in.Challenge.RandomData[0]%2 == 0

	if !showMeta {
		w.Header().Add("Refresh", fmt.Sprintf("%d; url=%s", in.Rule.Challenge.Difficulty+1, u.String()))
	}

	loc := localization.GetLocalizer(r)

	result := page(u.String(), in.Rule.Challenge.Difficulty, showMeta, loc)

	return result, nil
}

func (i *Impl) Validate(r *http.Request, lg *slog.Logger, in *challenge.ValidateInput) error {
	wantTime := in.Challenge.IssuedAt.Add(time.Duration(in.Rule.Challenge.Difficulty) * 800 * time.Millisecond)

	if time.Now().Before(wantTime) {
		return challenge.NewError("validate", "insufficent time", fmt.Errorf("%w: wanted user to wait until at least %s", challenge.ErrFailed, wantTime.Format(time.RFC3339)))
	}

	gotChallenge := r.FormValue("challenge")

	if subtle.ConstantTimeCompare([]byte(in.Challenge.RandomData), []byte(gotChallenge)) != 1 {
		return challenge.NewError("validate", "invalid response", fmt.Errorf("%w: wanted response %s but got %s", challenge.ErrFailed, in.Challenge.RandomData, gotChallenge))
	}

	return nil
}

package preact

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/a-h/templ"
)

//go:generate ./build.sh
//go:generate go tool github.com/a-h/templ/cmd/templ generate

//go:embed static/app.js
var appJS []byte

func renderAppJS(ctx context.Context, out io.Writer) error {
	fmt.Fprint(out, `<script type="module">`)
	out.Write(appJS)
	fmt.Fprint(out, "</script>")
	return nil
}

func init() {
	challenge.Register("preact", &impl{})
}

type impl struct{}

func (i *impl) Setup(mux *http.ServeMux) {}

func (i *impl) Issue(w http.ResponseWriter, r *http.Request, lg *slog.Logger, in *challenge.IssueInput) (templ.Component, error) {
	u, err := r.URL.Parse(anubis.BasePrefix + "/.within.website/x/cmd/anubis/api/pass-challenge")
	if err != nil {
		return nil, fmt.Errorf("can't render page: %w", err)
	}

	q := u.Query()
	q.Set("redir", r.URL.String())
	q.Set("id", in.Challenge.ID)
	u.RawQuery = q.Encode()

	loc := localization.GetLocalizer(r)

	result := page(u.String(), in.Challenge.RandomData, in.Rule.Challenge.Difficulty, loc)

	return result, nil
}

func (i *impl) Validate(r *http.Request, lg *slog.Logger, in *challenge.ValidateInput) error {
	wantTime := in.Challenge.IssuedAt.Add(time.Duration(in.Rule.Challenge.Difficulty) * 80 * time.Millisecond)

	if time.Now().Before(wantTime) {
		return challenge.NewError("validate", "insufficent time", fmt.Errorf("%w: wanted user to wait until at least %s", challenge.ErrFailed, wantTime.Format(time.RFC3339)))
	}

	got := r.FormValue("result")
	want := internal.SHA256sum(in.Challenge.RandomData)

	if subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
		return challenge.NewError("validate", "invalid response", fmt.Errorf("%w: wanted response %s but got %s", challenge.ErrFailed, want, got))
	}

	return nil
}

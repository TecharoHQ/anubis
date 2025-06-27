package proofofwork

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	chall "github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/web"
	"github.com/a-h/templ"
)

func init() {
	chall.Register("fast", &Impl{Algorithm: "fast"})
	chall.Register("slow", &Impl{Algorithm: "slow"})
}

type Impl struct {
	Algorithm string
}

func (i *Impl) Setup(mux *http.ServeMux) {
	/* no implementation required */
}

func (i *Impl) Issue(r *http.Request, lg *slog.Logger, in *chall.IssueInput) (templ.Component, error) {
	loc := localization.GetLocalizer(r)
	component, err := web.BaseWithChallengeAndOGTags(loc.T("making_sure_not_bot"), web.Index(loc), in.Impressum, in.Challenge, in.Rule.Challenge, in.OGTags, loc)
	if err != nil {
		return nil, fmt.Errorf("can't render page: %w", err)
	}

	return component, nil
}

func (i *Impl) Validate(r *http.Request, lg *slog.Logger, rule *policy.Bot, challenge string) error {
	nonceStr := r.FormValue("nonce")
	if nonceStr == "" {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w nonce", chall.ErrMissingField))
	}

	nonce, err := strconv.Atoi(nonceStr)
	if err != nil {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w: nonce: %w", chall.ErrInvalidFormat, err))

	}

	elapsedTimeStr := r.FormValue("elapsedTime")
	if elapsedTimeStr == "" {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w elapsedTime", chall.ErrMissingField))
	}

	elapsedTime, err := strconv.ParseFloat(elapsedTimeStr, 64)
	if err != nil {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w: elapsedTime: %w", chall.ErrInvalidFormat, err))
	}

	response := r.FormValue("response")
	if response == "" {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w response", chall.ErrMissingField))
	}

	calcString := fmt.Sprintf("%s%d", challenge, nonce)
	calculated := internal.SHA256sum(calcString)

	if subtle.ConstantTimeCompare([]byte(response), []byte(calculated)) != 1 {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w: wanted response %s but got %s", chall.ErrFailed, calculated, response))
	}

	// compare the leading zeroes
	if !strings.HasPrefix(response, strings.Repeat("0", rule.Challenge.Difficulty)) {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w: wanted %d leading zeros but got %s", chall.ErrFailed, rule.Challenge.Difficulty, response))
	}

	lg.Debug("challenge took", "elapsedTime", elapsedTime)
	chall.TimeTaken.WithLabelValues(i.Algorithm).Observe(elapsedTime)

	return nil
}

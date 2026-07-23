package proofofwork

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	chall "github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/a-h/templ"
)

//go:generate go tool github.com/a-h/templ/cmd/templ generate

func init() {
	chall.Register("fast", &Impl{Algorithm: "fast"})
	chall.Register("slow", &Impl{Algorithm: "slow"})
}

type Impl struct {
	Algorithm string
}

func (i *Impl) Setup(mux *http.ServeMux) {}

func (i *Impl) Issue(w http.ResponseWriter, r *http.Request, lg *slog.Logger, in *chall.IssueInput) (templ.Component, error) {
	loc := localization.GetLocalizer(r)
	return page(loc), nil
}

func (i *Impl) Validate(r *http.Request, lg *slog.Logger, in *chall.ValidateInput) error {
	if err := in.Valid(); err != nil {
		return chall.NewError("validate", "invalid input", err)
	}

	rule := in.Rule
	challenge := in.Challenge.RandomData

	nonceStr := r.FormValue("nonce")
	if nonceStr == "" {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w nonce", chall.ErrMissingField))
	}

	_, err := strconv.Atoi(nonceStr)
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

	// Stream the challenge and nonce into a single sha256 hasher to avoid
	// the intermediate "challenge + nonceStr" concatenation. Hex-encode
	// the digest into a stack buffer so the comparison runs without
	// allocating a heap string.
	h := sha256.New()
	h.Write([]byte(challenge))
	h.Write([]byte(nonceStr))
	var sumBuf [sha256.Size]byte
	sum := h.Sum(sumBuf[:0])
	var hexBuf [sha256.Size * 2]byte
	hex.Encode(hexBuf[:], sum)

	if subtle.ConstantTimeCompare([]byte(response), hexBuf[:]) != 1 {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w: wanted response %s but got %s", chall.ErrFailed, string(hexBuf[:]), response))
	}

	// compare the leading zeroes
	if !strings.HasPrefix(response, strings.Repeat("0", rule.Challenge.Difficulty)) {
		return chall.NewError("validate", "invalid response", fmt.Errorf("%w: wanted %d leading zeros but got %s", chall.ErrFailed, rule.Challenge.Difficulty, response))
	}

	lg.DebugContext(r.Context(), "challenge took", "elapsedTime", elapsedTime)
	chall.TimeTaken.WithLabelValues(i.Algorithm).Observe(elapsedTime)

	return nil
}

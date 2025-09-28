package wasm

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	chall "github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/localization"
	"github.com/TecharoHQ/anubis/wasm"
	"github.com/TecharoHQ/anubis/web"
	"github.com/a-h/templ"
)

//go:generate go tool github.com/a-h/templ/cmd/templ generate

func init() {
	chall.Register("hashx", &Impl{algorithm: "argon2id"})
	chall.Register("sha256", &Impl{algorithm: "sha256"})
}

type Impl struct {
	algorithm string
	runner    *wasm.Runner
}

func (i *Impl) Setup(mux *http.ServeMux) error {
	fname := fmt.Sprintf("static/wasm/simd128/%s.wasm", i.algorithm)
	fin, err := web.Static.Open(fname)
	if err != nil {
		return err
	}
	defer fin.Close()

	i.runner, err = wasm.NewRunner(context.Background(), fname, fin)

	return err
}

func (i *Impl) Issue(w http.ResponseWriter, r *http.Request, lg *slog.Logger, in *chall.IssueInput) (templ.Component, error) {
	loc := localization.GetLocalizer(r)
	return page(loc), nil
}

func (i *Impl) Validate(r *http.Request, lg *slog.Logger, in *chall.ValidateInput) error {
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

	challengeBytes, err := hex.DecodeString(in.Challenge.RandomData)
	if err != nil {
		return chall.NewError("validate", "invalid random data", fmt.Errorf("can't decode random data: %w", err))
	}

	gotBytes, err := hex.DecodeString(response)
	if err != nil {
		return chall.NewError("validate", "invalid client data format", fmt.Errorf("%w response", chall.ErrInvalidFormat))
	}

	ok, err := i.runner.Verify(r.Context(), challengeBytes, gotBytes, uint32(nonce), uint32(in.Rule.Challenge.Difficulty))
	if err != nil {
		return chall.NewError("validate", "internal WASM error", fmt.Errorf("can't run wasm validation logic: %w", err))
	}

	if !ok {
		return chall.NewError("verify", "client calculated wrong data", fmt.Errorf("%w: response invalid: %s", chall.ErrFailed, response))
	}

	lg.Debug("challenge took", "elapsedTime", elapsedTime)
	chall.TimeTaken.WithLabelValues(i.algorithm).Observe(elapsedTime)

	return nil
}

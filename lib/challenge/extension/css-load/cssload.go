package cssload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/challenge/extension"
	"github.com/TecharoHQ/anubis/lib/store"
	"github.com/a-h/templ"
	"github.com/google/uuid"
)

const (
	validPrefix = "ext:css-load:valid:"
	storePrefix = "ext:css-load:seen:"
)

func init() {
	extension.Register("css-load", &Impl{})
}

type Impl struct {
	st store.Interface
}

func (i *Impl) cssFolder() string {
	return anubis.BasePrefix + "/.within.website/x/cmd/anubis/ext/css-load"
}

func (i *Impl) Setup(mux *http.ServeMux, st store.Interface) error {
	i.st = st

	mux.HandleFunc(fmt.Sprintf("GET %s/{challengeID}", i.cssFolder()), i.renderCSS)

	return nil
}

func (i *Impl) Head(r *http.Request, chall *challenge.Challenge) templ.Component {
	key := chall.ID

	validCh := store.JSON[struct{}]{Underlying: i.st, Prefix: validPrefix}

	if _, err := validCh.Get(r.Context(), key); errors.Is(err, store.ErrNotFound) {
		validCh.Set(context.WithoutCancel(r.Context()), key, struct{}{}, 30*time.Minute) //nolint:errcheck
	}

	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := fmt.Fprintf(w, `<style>@import url("%s/%s");</style>`, i.cssFolder(), key)
		return err
	})
}

func (i *Impl) Validate(r *http.Request, lg *slog.Logger, in *challenge.ValidateInput) error {
	seen := store.JSON[struct{}]{Underlying: i.st, Prefix: storePrefix}
	_, err := seen.Get(r.Context(), in.Challenge.ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return challenge.NewError("css-load", "Please ensure your browser has modern web standards enabled", fmt.Errorf("%w: CSS was not fetched", challenge.ErrFailed))
	case err != nil:
		lg.DebugContext(r.Context(), "store unavailable", "err", err)
		return nil // fail open
	}
	return nil
}

func (i *Impl) renderCSS(w http.ResponseWriter, r *http.Request) {
	validCh := store.JSON[struct{}]{Underlying: i.st, Prefix: validPrefix}
	seen := store.JSON[struct{}]{Underlying: i.st, Prefix: storePrefix}
	chID := r.PathValue("challengeID")

	chIDo, err := uuid.Parse(chID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if _, err := validCh.Get(r.Context(), chIDo.String()); err != nil {
		http.NotFound(w, r)
		return
	}

	seen.Set(r.Context(), chIDo.String(), struct{}{}, 30*time.Minute) //nolint:errcheck

	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusOK)
}

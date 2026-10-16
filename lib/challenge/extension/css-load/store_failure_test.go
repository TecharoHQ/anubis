package cssload

import (
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/store"
)

type unavailableStore struct{ store.Interface }

func (unavailableStore) Get(context.Context, string) ([]byte, error) { return nil, store.ErrCantDecode }

func TestValidateRejectsUnavailableStore(t *testing.T) {
	impl := &Impl{st: unavailableStore{}}
	err := impl.Validate(httptest.NewRequest("GET", "/", nil), slog.Default(), &challenge.ValidateInput{Challenge: &challenge.Challenge{ID: "test"}})
	if !errors.Is(err, store.ErrCantDecode) {
		t.Fatalf("store failure accepted or hidden: %v", err)
	}
}

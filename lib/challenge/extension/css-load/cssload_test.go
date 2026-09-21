package cssload

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/store/memory"
	"github.com/google/uuid"
	"github.com/neilotoole/slogt/v2"
)

func TestCSSLoad(t *testing.T) {
	st := memory.New(t.Context())
	mux := http.NewServeMux()
	i := &Impl{}

	if err := i.Setup(mux, st); err != nil {
		t.Fatalf("can't setup css-load extension: %v", err)
	}

	chID := uuid.Must(uuid.NewV7())
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	comp := i.Head(r, &challenge.Challenge{ID: chID.String()})
	if err := comp.Render(t.Context(), io.Discard); err != nil {
		t.Fatal(err)
	}

	lg := slogt.New(t)

	t.Run("load-css", func(t *testing.T) {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, i.cssFolder()+"/"+chID.String(), nil)
		rw := httptest.NewRecorder()

		mux.ServeHTTP(rw, req)

		resp := rw.Result()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("wanted status code %d, got: %s", http.StatusOK, resp.Status)
		}
	})

	t.Run("validate", func(t *testing.T) {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		if err := i.Validate(r, lg, &challenge.ValidateInput{
			Rule:      nil,
			Challenge: &challenge.Challenge{ID: chID.String()},
			Store:     st,
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("validate-invalid", func(t *testing.T) {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		if err := i.Validate(r, lg, &challenge.ValidateInput{
			Rule:      nil,
			Challenge: &challenge.Challenge{ID: "hunter2"},
			Store:     st,
		}); err == nil {
			t.Fatal("validation passed when it should have failed")
		}
	})
}

func FuzzRenderCSS(f *testing.F) {
	st := memory.New(f.Context())
	mux := http.NewServeMux()
	i := &Impl{}

	if err := i.Setup(mux, st); err != nil {
		f.Fatalf("can't setup css-load extension: %v", err)
	}

	f.Add(uuid.Must(uuid.NewV7()).String())

	f.Fuzz(func(t *testing.T, a string) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, i.cssFolder()+"/"+a, nil)
		if err != nil {
			return
		}
		rw := httptest.NewRecorder()

		mux.ServeHTTP(rw, req)
	})
}

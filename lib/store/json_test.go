package store_test

import (
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/store"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

func TestJSON(t *testing.T) {
	type data struct {
		ID string `json:"id"`
	}

	st := memory.New(t.Context())
	db := store.JSON[data]{
		Underlying: st,
		Prefix:     "foo:",
	}

	if err := db.Set(t.Context(), "test", data{ID: t.Name()}, time.Minute); err != nil {
		t.Fatal(err)
	}

	got, err := db.Get(t.Context(), "test")
	if err != nil {
		t.Fatal(err)
	}

	if got.ID != t.Name() {
		t.Fatalf("got wrong data for key \"test\", wanted %q but got: %q", t.Name(), got.ID)
	}

	if err := db.Delete(t.Context(), "test"); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Get(t.Context(), "test"); err == nil {
		t.Fatal("wanted invalid get to fail, it did not")
	}

	if err := st.Set(t.Context(), "foo:test", []byte("}"), time.Minute); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Get(t.Context(), "test"); err == nil {
		t.Fatal("wanted invalid get to fail, it did not")
	}
}

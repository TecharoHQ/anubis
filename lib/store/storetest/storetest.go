package storetest

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/store"
)

func Common(t *testing.T, f store.Factory, config json.RawMessage) {
	if err := f.Valid(config); err != nil {
		t.Fatal(err)
	}

	s, err := f.Build(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		err  error
		doer func(t *testing.T, s store.Interface) error
		name string
	}{
		{
			name: "basic get set delete",
			doer: func(t *testing.T, s store.Interface) error {
				if _, err := s.Get(t.Context(), t.Name()); !errors.Is(err, store.ErrNotFound) {
					t.Errorf("wanted %s to not exist in store but it exists anyways", t.Name())
				}

				if err := s.Set(t.Context(), t.Name(), []byte(t.Name()), 5*time.Minute); err != nil {
					return err
				}

				val, err := s.Get(t.Context(), t.Name())
				if errors.Is(err, store.ErrNotFound) {
					t.Errorf("wanted %s to exist in store but it does not: %v", t.Name(), err)
				} else if err != nil {
					t.Error(err)
				}

				if !bytes.Equal(val, []byte(t.Name())) {
					t.Logf("want: %q", t.Name())
					t.Logf("got:  %q", string(val))
					t.Error("wrong value returned")
				}

				if err := s.Delete(t.Context(), t.Name()); err != nil {
					return err
				}

				if _, err := s.Get(t.Context(), t.Name()); !errors.Is(err, store.ErrNotFound) {
					t.Error("wanted test to not exist in store but it exists anyways")
				}

				if err := s.Delete(t.Context(), t.Name()); err == nil {
					t.Errorf("key %q does not exist and Delete did not return non-nil", t.Name())
				}

				return nil
			},
		},
		{
			name: "expires",
			doer: func(t *testing.T, s store.Interface) error {
				if err := s.Set(t.Context(), t.Name(), []byte(t.Name()), 150*time.Millisecond); err != nil {
					return err
				}

				//nosleep:bypass XXX(Xe): use Go's time faking thing in Go 1.25 when that is released.
				time.Sleep(155 * time.Millisecond)

				if _, err := s.Get(t.Context(), t.Name()); !errors.Is(err, store.ErrNotFound) {
					t.Errorf("wanted %s to not exist in store but it exists anyways", t.Name())
				}

				return nil
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.doer(t, s); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Error("wrong error")
			}
		})
	}
}

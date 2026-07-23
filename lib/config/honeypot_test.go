package config

import (
	"errors"
	"testing"
)

func TestHoneypotValid(t *testing.T) {
	for _, cs := range []struct {
		err  error
		name string
		inp  Honeypot
	}{
		{
			name: "naive enabled",
			inp: Honeypot{
				Enabled:        true,
				Implementation: "naive",
			},
			err: nil,
		},
		{
			name: "naive disabled",
			inp: Honeypot{
				Enabled:        false,
				Implementation: "naive",
			},
			err: nil,
		},
		{
			name: "empty implementation",
			inp: Honeypot{
				Enabled:        true,
				Implementation: "",
			},
			err: ErrInvalidHoneypotMethod,
		},
		{
			name: "unknown implementation",
			inp: Honeypot{
				Enabled:        true,
				Implementation: "sophisticated",
			},
			err: ErrInvalidHoneypotMethod,
		},
		{
			name: "wrong case implementation",
			inp: Honeypot{
				Enabled:        true,
				Implementation: "Naive",
			},
			err: ErrInvalidHoneypotMethod,
		},
		{
			name: "zero value",
			inp:  Honeypot{},
			err:  nil,
		},
	} {
		t.Run(cs.name, func(t *testing.T) {
			if err := cs.inp.Valid(); !errors.Is(err, cs.err) {
				t.Logf("want: %v", cs.err)
				t.Logf("got:  %v", err)
				t.Error("validation returned an unexpected error")
			}
		})
	}
}

func TestHoneypotDefault(t *testing.T) {
	want := Honeypot{
		Enabled:        true,
		Implementation: "naive",
	}

	got := (Honeypot{}).Default()
	if got != want {
		t.Errorf("Default() = %+v, want %+v", got, want)
	}
}

func TestHoneypotDefaultIsValid(t *testing.T) {
	h := (Honeypot{}).Default()
	if err := h.Valid(); err != nil {
		t.Fatal(err)
	}
}

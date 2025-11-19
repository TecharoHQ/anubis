package config_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/bbolt"
	"github.com/TecharoHQ/anubis/lib/store/valkey"
)

func TestStoreValid(t *testing.T) {
	for _, tt := range []struct {
		err   error
		name  string
		input config.Store
	}{
		{
			name:  "no backend",
			input: config.Store{},
			err:   config.ErrNoStoreBackend,
		},
		{
			name: "in-memory backend",
			input: config.Store{
				Backend: "memory",
			},
		},
		{
			name: "bbolt backend",
			input: config.Store{
				Backend:    "bbolt",
				Parameters: json.RawMessage(`{"path": "/tmp/foo", "bucket": "bar"}`),
			},
		},
		{
			name: "valkey backend",
			input: config.Store{
				Backend:    "valkey",
				Parameters: json.RawMessage(`{"url": "redis://valkey:6379/0"}`),
			},
		},
		{
			name: "valkey backend no URL",
			input: config.Store{
				Backend:    "valkey",
				Parameters: json.RawMessage(`{}`),
			},
			err: valkey.ErrNoURL,
		},
		{
			name: "valkey backend bad URL",
			input: config.Store{
				Backend:    "valkey",
				Parameters: json.RawMessage(`{"url": "http://anubis.techaro.lol"}`),
			},
			err: valkey.ErrBadURL,
		},
		{
			name: "bbolt backend no path",
			input: config.Store{
				Backend:    "bbolt",
				Parameters: json.RawMessage(`{"path": "", "bucket": "bar"}`),
			},
			err: bbolt.ErrMissingPath,
		},
		{
			name: "unknown backend",
			input: config.Store{
				Backend: "taco salad",
			},
			err: config.ErrUnknownStoreBackend,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.input.Valid(); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Error("invalid error returned")
			}
		})
	}
}

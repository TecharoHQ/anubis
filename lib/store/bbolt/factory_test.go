package bbolt

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestFactoryValid(t *testing.T) {
	f := Factory{}

	t.Run("bad config", func(t *testing.T) {
		if err := f.Valid(json.RawMessage(`}`)); err == nil {
			t.Error("wanted parsing failure but got a successful result")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		for _, tt := range []struct {
			err  error
			name string
			cfg  Config
		}{
			{
				name: "missing path",
				cfg:  Config{},
				err:  ErrMissingPath,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				data, err := json.Marshal(tt.cfg)
				if err != nil {
					t.Fatal(err)
				}

				if err := f.Valid(json.RawMessage(data)); !errors.Is(err, tt.err) {
					t.Error(err)
				}
			})
		}
	})
}

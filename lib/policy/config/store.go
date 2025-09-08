package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TecharoHQ/anubis/lib/store"
	_ "github.com/TecharoHQ/anubis/lib/store/all"
)

var (
	ErrNoStoreBackend      = errors.New("config.Store: no backend defined")
	ErrUnknownStoreBackend = errors.New("config.Store: unknown backend")
)

type Store struct {
	Backend    string          `json:"backend"`
	Parameters json.RawMessage `json:"parameters"`
}

func (s *Store) Valid() error {
	var errs []error

	if len(s.Backend) == 0 {
		errs = append(errs, ErrNoStoreBackend)
	}

	fac, ok := store.Get(s.Backend)
	switch ok {
	case true:
		if err := fac.Valid(s.Parameters); err != nil {
			errs = append(errs, err)
		}
	case false:
		errs = append(errs, fmt.Errorf("%w: %q", ErrUnknownStoreBackend, s.Backend))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

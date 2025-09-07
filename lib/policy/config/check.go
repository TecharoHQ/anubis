//go:build ignore

package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TecharoHQ/anubis/lib/checker"
)

var (
	ErrUnknownCheckType = errors.New("config.Bot.Check: unknown check type")
)

type AllChecks struct {
	All []Check `json:"all"`
}

type AnyChecks struct {
	All []Check `json:"any"`
}

type Check struct {
	Type string          `json:"type"`
	Args json.RawMessage `json:"args"`
}

func (c *Check) Valid(ctx context.Context) error {
	var errs []error

	if len(c.Type) == 0 {
		errs = append(errs, ErrNoStoreBackend)
	}

	fac, ok := checker.Get(c.Type)
	switch ok {
	case true:
		if err := fac.Valid(ctx, c.Args); err != nil {
			errs = append(errs, err)
		}
	case false:
		errs = append(errs, fmt.Errorf("%w: %q", ErrUnknownCheckType, c.Type))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

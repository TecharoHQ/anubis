package config

import (
	"errors"
	"fmt"
)

var ErrInvalidHoneypotMethod = errors.New("config.Honeypot: invalid implementation selected (try: \"naive\")")

type Honeypot struct {
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	Implementation string `json:"implementation" yaml:"implementation"`
}

func (h Honeypot) Valid() error {
	var errs []error

	// Short-circuit, if disabled then the implementation is moot.
	if !h.Enabled {
		return nil
	}

	if h.Implementation == "" {
		errs = append(errs, fmt.Errorf("config.Honeypot: missing field \"implementation\": %w", ErrMissingValue))
	}

	if h.Implementation != "naive" {
		errs = append(errs, fmt.Errorf("%w: got: %q", ErrInvalidHoneypotMethod, h.Implementation))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (h Honeypot) Default() Honeypot {
	return Honeypot{
		Enabled:        true,
		Implementation: "naive",
	}
}

package config

import (
	"errors"
	"fmt"
	"net"
)

var (
	ErrInvalidHostpost = errors.New("bind: invalid host:port")
)

type Bind struct {
	HTTP    string `hcl:"http"`
	HTTPS   string `hcl:"https"`
	Metrics string `hcl:"metrics"`
}

func (b *Bind) Valid() error {
	var errs []error

	if _, _, err := net.SplitHostPort(b.HTTP); err != nil {
		errs = append(errs, fmt.Errorf("%w %q: %w", ErrInvalidHostpost, b.HTTP, err))
	}

	if _, _, err := net.SplitHostPort(b.HTTPS); err != nil {
		errs = append(errs, fmt.Errorf("%w %q: %w", ErrInvalidHostpost, b.HTTPS, err))
	}

	if _, _, err := net.SplitHostPort(b.Metrics); err != nil {
		errs = append(errs, fmt.Errorf("%w %q: %w", ErrInvalidHostpost, b.Metrics, err))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

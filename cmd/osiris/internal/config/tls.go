package config

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
)

var (
	ErrCantReadTLS       = errors.New("tls: can't read TLS")
	ErrInvalidTLSKeypair = errors.New("tls: can't parse TLS keypair")
)

type TLS struct {
	Cert string `hcl:"cert"`
	Key  string `hcl:"key"`
}

func (t TLS) Valid() error {
	var errs []error

	if _, err := os.Stat(t.Cert); err != nil {
		errs = append(errs, fmt.Errorf("%w certificate %s: %w", ErrCantReadTLS, t.Cert, err))
	}

	if _, err := os.Stat(t.Key); err != nil {
		errs = append(errs, fmt.Errorf("%w key %s: %w", ErrCantReadTLS, t.Key, err))
	}

	if _, err := tls.LoadX509KeyPair(t.Cert, t.Key); err != nil {
		errs = append(errs, fmt.Errorf("%w (%s, %s): %w", ErrInvalidTLSKeypair, t.Cert, t.Key, err))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

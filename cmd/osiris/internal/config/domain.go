package config

import (
	"errors"
	"fmt"
	"net/url"

	"golang.org/x/net/idna"
)

var (
	ErrInvalidDomainName      = errors.New("domain: name is invalid")
	ErrInvalidDomainTLSConfig = errors.New("domain: TLS config is invalid")
	ErrInvalidURL             = errors.New("invalid URL")
	ErrInvalidURLScheme       = errors.New("URL has invalid scheme")
)

type Domain struct {
	Name               string `hcl:"name,label"`
	TLS                TLS    `hcl:"tls,block"`
	Target             string `hcl:"target"`
	InsecureSkipVerify bool   `hcl:"insecure_skip_verify,optional"`
	HealthTarget       string `hcl:"health_target"`
}

func (d Domain) Valid() error {
	var errs []error

	if _, err := idna.Lookup.ToASCII(d.Name); err != nil {
		errs = append(errs, fmt.Errorf("%w %q: %w", ErrInvalidDomainName, d.Name, err))
	}

	if err := d.TLS.Valid(); err != nil {
		errs = append(errs, fmt.Errorf("%w: %w", ErrInvalidDomainTLSConfig, err))
	}

	if err := isURLValid(d.Target); err != nil {
		errs = append(errs, fmt.Errorf("target has %w %q: %w", ErrInvalidURL, d.Target, err))
	}

	if err := isURLValid(d.HealthTarget); err != nil {
		errs = append(errs, fmt.Errorf("health_target has %w %q: %w", ErrInvalidURL, d.HealthTarget, err))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

func isURLValid(input string) error {
	u, err := url.Parse(input)
	if err != nil {
		return err
	}

	switch u.Scheme {
	case "http", "https", "h2c", "unix":
		// do nothing
	default:
		return fmt.Errorf("%w %s has scheme %s (want http, https, h2c, unix)", ErrInvalidURLScheme, input, u.Scheme)
	}

	return nil
}

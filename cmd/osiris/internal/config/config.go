package config

import (
	"errors"
	"fmt"
)

type Toplevel struct {
	Bind    Bind     `hcl:"bind,block"`
	Domains []Domain `hcl:"domain,block"`
}

func (t *Toplevel) Valid() error {
	var errs []error

	if err := t.Bind.Valid(); err != nil {
		errs = append(errs, fmt.Errorf("invalid bind block:\n%w", err))
	}

	for _, d := range t.Domains {
		if err := d.Valid(); err != nil {
			errs = append(errs, fmt.Errorf("when parsing domain %s: %w", d.Name, err))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("invalid configuration file:\n%w", errors.Join(errs...))
	}

	return nil
}

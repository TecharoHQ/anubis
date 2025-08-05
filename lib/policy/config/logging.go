package config

import (
	"errors"
	"fmt"
)

type Logging struct {
	Filters []LogFilter `json:"filters,omitempty" yaml:"filters,omitempty"`
}

func (l *Logging) Valid() error {
	var errs []error

	for _, lf := range l.Filters {
		if err := lf.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

type LogFilter struct {
	Name       string           `json:"name" yaml:"name"`
	Expression ExpressionOrList `json:"expression" yaml:"expression"`
}

func (lf LogFilter) Valid() error {
	var errs []error

	if lf.Name == "" {
		errs = append(errs, fmt.Errorf("%w: log filter has no name", ErrMissingValue))
	}

	if err := lf.Expression.Valid(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return fmt.Errorf("log filter %q is not valid: %w", lf.Name, errors.Join(errs...))
	}

	return nil
}

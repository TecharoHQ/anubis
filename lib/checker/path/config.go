package path

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrNoRegex      = errors.New("path: no regex is configured")
	ErrInvalidRegex = errors.New("path: regex is invalid")
)

type fileConfig struct {
	Regex string `json:"regex" yaml:"regex"`
}

func (fc fileConfig) String() string {
	return fmt.Sprintf("regex=%q", fc.Regex)
}

func (fc fileConfig) Valid() error {
	var errs []error

	if fc.Regex == "" {
		errs = append(errs, ErrNoRegex)
	}

	if _, err := regexp.Compile(fc.Regex); err != nil {
		errs = append(errs, ErrInvalidRegex, err)
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

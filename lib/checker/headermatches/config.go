package headermatches

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrNoHeader     = errors.New("headermatches: no header is configured")
	ErrNoValueRegex = errors.New("headermatches: no value regex is configured")
	ErrInvalidRegex = errors.New("headermatches: value regex is invalid")
)

type fileConfig struct {
	Header     string `json:"header" yaml:"header"`
	ValueRegex string `json:"value_regex" yaml:"value_regex"`
}

func (fc fileConfig) String() string {
	return fmt.Sprintf("header=%q value_regex=%q", fc.Header, fc.ValueRegex)
}

func (fc fileConfig) Valid() error {
	var errs []error

	if fc.Header == "" {
		errs = append(errs, ErrNoHeader)
	}

	if fc.ValueRegex == "" {
		errs = append(errs, ErrNoValueRegex)
	}

	if _, err := regexp.Compile(fc.ValueRegex); err != nil {
		errs = append(errs, ErrInvalidRegex, err)
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

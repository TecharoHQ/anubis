package config

import (
	"context"
	"errors"
	"fmt"
	"io"
)

var ErrMissingValue = errors.New("config: missing value")

type Impressum struct {
	Footer string        `json:"footer" yaml:"footer"`
	Page   ImpressumPage `json:"page" yaml:"page"`
}

func (i Impressum) Render(_ context.Context, w io.Writer) error {
	if _, err := fmt.Fprint(w, i.Footer); err != nil {
		return err
	}
	return nil
}

func (i Impressum) Valid() error {
	var errs []error

	if len(i.Footer) == 0 {
		errs = append(errs, fmt.Errorf("%w: impressum footer must be defined", ErrMissingValue))
	}

	if err := i.Page.Valid(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

type ImpressumPage struct {
	Title string `json:"title" yaml:"title"`
	Body  string `json:"body" yaml:"body"`
}

func (ip ImpressumPage) Render(_ context.Context, w io.Writer) error {
	if _, err := fmt.Fprint(w, ip.Body); err != nil {
		return err
	}

	return nil
}

func (ip ImpressumPage) Valid() error {
	var errs []error

	if len(ip.Title) == 0 {
		errs = append(errs, fmt.Errorf("%w: impressum page title must be defined", ErrMissingValue))
	}

	if len(ip.Body) == 0 {
		errs = append(errs, fmt.Errorf("%w: impressum body title must be defined", ErrMissingValue))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

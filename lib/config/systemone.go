package config

import (
	"errors"
	"fmt"
	"net/url"
)

var (
	ErrSystemOneInvalidURL = errors.New("config.SystemOne: invalid API URL")
)

type SystemOne struct {
	URL       string            `json:"url" yaml:"url"`
	Model     string            `json:"model" yaml:"model"`
	Context   map[string]string `json:"context" yaml:"context"`
	APIKeyEnv string            `json:"api_key_env" yaml:"api_key_env"`
}

func (s *SystemOne) Valid() error {
	var errs []error

	switch s.URL == "" {
	case true:
	case false:
		_, err := url.Parse(s.URL)
		if err != nil {
			errs = append(errs, fmt.Errorf("%w: %w", ErrSystemOneInvalidURL, err))
		}
	}

	if s.Model == "" {
		errs = append(errs, fmt.Errorf("%w: config.SystemOne: no model defined", ErrMissingValue))
	}

	if s.APIKeyEnv == "" {
		errs = append(errs, fmt.Errorf("%w: config.SystemOne: no API key envvar defined", ErrMissingValue))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

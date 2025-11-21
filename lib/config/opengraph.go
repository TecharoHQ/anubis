package config

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidOpenGraphConfig   = errors.New("config.OpenGraph: invalid OpenGraph configuration")
	ErrOpenGraphTTLDoesNotParse = errors.New("config.OpenGraph: ttl does not parse as a Duration, see https://pkg.go.dev/time#ParseDuration (formatted like 5m -> 5 minutes, 2h -> 2 hours, etc)")
	ErrOpenGraphMissingProperty = errors.New("config.OpenGraph: default opengraph tags missing a property")
)

type openGraphFileConfig struct {
	Override     map[string]string `json:"override,omitempty" yaml:"override,omitempty"`
	TimeToLive   string            `json:"ttl" yaml:"ttl"`
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	ConsiderHost bool              `json:"considerHost" yaml:"enabled"`
}

type OpenGraph struct {
	Override     map[string]string `json:"override,omitempty" yaml:"override,omitempty"`
	TimeToLive   time.Duration     `json:"ttl" yaml:"ttl"`
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	ConsiderHost bool              `json:"considerHost" yaml:"enabled"`
}

func (og *openGraphFileConfig) Valid() error {
	var errs []error

	if _, err := time.ParseDuration(og.TimeToLive); err != nil {
		errs = append(errs, fmt.Errorf("%w: ParseDuration(%q) returned: %w", ErrOpenGraphTTLDoesNotParse, og.TimeToLive, err))
	}

	if len(og.Override) != 0 {
		for _, tag := range []string{
			"og:title",
		} {
			if _, ok := og.Override[tag]; !ok {
				errs = append(errs, fmt.Errorf("%w: %s", ErrOpenGraphMissingProperty, tag))
			}
		}
	}

	if len(errs) != 0 {
		return errors.Join(ErrInvalidOpenGraphConfig, errors.Join(errs...))
	}

	return nil
}

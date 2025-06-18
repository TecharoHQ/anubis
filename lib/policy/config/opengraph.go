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
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	ConsiderHost bool              `json:"considerHost" yaml:"enabled"`
	TimeToLive   string            `json:"ttl" yaml:"ttl"`
	Default      map[string]string `json:"default" yaml:"default"`
}

type OpenGraph struct {
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	ConsiderHost bool              `json:"considerHost" yaml:"enabled"`
	Default      map[string]string `json:"default" yaml:"default"`
	TimeToLive   time.Duration     `json:"ttl" yaml:"ttl"`
}

func (og *openGraphFileConfig) Valid() error {
	var errs []error

	if _, err := time.ParseDuration(og.TimeToLive); err != nil {
		errs = append(errs, fmt.Errorf("%w: ParseDuration(%q) returned: %w", ErrOpenGraphTTLDoesNotParse, og.TimeToLive, err))
	}

	if len(og.Default) != 0 {
		for _, tag := range []string{
			"og:title",
		} {
			if _, ok := og.Default[tag]; !ok {
				errs = append(errs, fmt.Errorf("%w: %s", ErrOpenGraphMissingProperty, tag))
			}
		}
	}

	if len(errs) != 0 {
		return errors.Join(ErrInvalidOpenGraphConfig, errors.Join(errs...))
	}

	return nil
}

package config

import (
	"errors"
	"testing"
)

func TestOpenGraphFileConfigValid(t *testing.T) {
	for _, tt := range []struct {
		err   error
		input *openGraphFileConfig
		name  string
	}{
		{
			name: "basic happy path",
			input: &openGraphFileConfig{
				Enabled:      true,
				ConsiderHost: false,
				TimeToLive:   "1h",
				Override:     map[string]string{},
			},
			err: nil,
		},
		{
			name: "basic happy path with default",
			input: &openGraphFileConfig{
				Enabled:      true,
				ConsiderHost: false,
				TimeToLive:   "1h",
				Override: map[string]string{
					"og:title": "foobar",
				},
			},
			err: nil,
		},
		{
			name: "invalid time duration",
			input: &openGraphFileConfig{
				Enabled:      true,
				ConsiderHost: false,
				TimeToLive:   "taco",
				Override:     map[string]string{},
			},
			err: ErrOpenGraphTTLDoesNotParse,
		},
		{
			name: "missing og:title in defaults",
			input: &openGraphFileConfig{
				Enabled:      true,
				ConsiderHost: false,
				TimeToLive:   "1h",
				Override: map[string]string{
					"description": "foobar",
				},
			},
			err: ErrOpenGraphMissingProperty,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.input.Valid(); !errors.Is(err, tt.err) {
				t.Logf("wanted error: %v", tt.err)
				t.Logf("got error:    %v", err)
				t.Error("validation failed")
			}
		})
	}
}

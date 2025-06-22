package config

import (
	"errors"
	"testing"
)

func TestGeoIPValid(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input *GeoIP
		err   error
	}{
		{
			name: "basic valid",
			input: &GeoIP{
				Countries: []string{"CA"},
			},
		},
		{
			name: "invalid country",
			input: &GeoIP{
				Countries: []string{"XOB"},
			},
			err: ErrNotCountryCode,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.input.Valid(); !errors.Is(err, tt.err) {
				t.Logf("want: %v", tt.err)
				t.Logf("got:  %v", err)
				t.Error("got wrong validation error")
			}
		})
	}
}

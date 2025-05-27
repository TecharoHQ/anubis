package config

import (
	"errors"
	"testing"
)

func TestGeoIPValid(t *testing.T) {
	for _, cs := range []struct {
		name      string
		countries []string
		err       error
	}{
		{
			name:      "basic-working",
			countries: []string{"US", "Ca", "mx"},
			err:       nil,
		},
	} {
		t.Run(cs.name, func(t *testing.T) {
			g := &GeoIP{
				Countries: cs.countries,
			}
			err := g.Valid()
			if !errors.Is(err, cs.err) {
				t.Fatalf("wanted error %v but got: %v", cs.err, err)
			}
			if err == nil && cs.err != nil {
				t.Fatalf("wanted error %v but got none", cs.err)
			}
		})
	}
}

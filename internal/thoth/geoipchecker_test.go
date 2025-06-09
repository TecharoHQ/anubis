package thoth

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/policy/checker"
)

var _ checker.Impl = &ASNChecker{}

func TestGeoIPChecker(t *testing.T) {
	cli := loadSecrets(t)

	asnc := &GeoIPChecker{
		iptoasn: cli.iptoasn,
		countries: map[string]struct{}{
			"us": {},
		},
		hash: "foobar",
	}

	for _, cs := range []struct {
		ipAddress string
		wantMatch bool
		wantError bool
	}{
		{
			ipAddress: "1.1.1.1",
			wantMatch: true,
			wantError: false,
		},
		{
			ipAddress: "70.31.0.1",
			wantMatch: false,
			wantError: false,
		},
		{
			ipAddress: "taco",
			wantMatch: false,
			wantError: true,
		},
	} {
		t.Run(fmt.Sprintf("%v", cs), func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Real-Ip", cs.ipAddress)

			match, err := asnc.Check(req)

			if match != cs.wantMatch {
				t.Errorf("Wanted match: %v, got: %v", cs.wantMatch, match)
			}

			switch {
			case err != nil && !cs.wantError:
				t.Errorf("Did not want error but got: %v", err)
			case err == nil && cs.wantError:
				t.Error("Wanted error but got none")
			}
		})
	}
}

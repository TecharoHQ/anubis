package thoth_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/internal/thoth"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
)

var _ checker.Impl = &thoth.GeoIPChecker{}

func TestGeoIPChecker(t *testing.T) {
	cli := loadSecrets(t)

	asnc := cli.GeoIPCheckerFor([]string{"us"})

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
			ipAddress: "2.2.2.2",
			wantMatch: false,
			wantError: false,
		},
		{
			ipAddress: "taco",
			wantMatch: false,
			wantError: false,
		},
		{
			ipAddress: "127.0.0.1",
			wantMatch: false,
			wantError: false,
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

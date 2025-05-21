package thoth

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/policy"
	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
)

var _ policy.Checker = &ASNChecker{}

func TestASNChecker(t *testing.T) {
	cli := loadSecrets(t)

	asnc := &ASNChecker{
		iptoasn: cli.iptoasn,
		asns: map[uint32]struct{}{
			13335: {},
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
			ipAddress: "8.8.8.8",
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

func BenchmarkWithCache(b *testing.B) {
	cli := loadSecrets(b)
	req := &iptoasnv1.LookupRequest{IpAddress: "1.1.1.1"}

	_, err := cli.iptoasn.Lookup(b.Context(), req)
	if err != nil {
		b.Error(err)
	}

	for b.Loop() {
		_, err := cli.iptoasn.Lookup(b.Context(), req)
		if err != nil {
			b.Error(err)
		}
	}
}

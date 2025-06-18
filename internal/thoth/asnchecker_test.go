package thoth_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/internal/thoth"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
)

var _ checker.Impl = &thoth.ASNChecker{}

func TestASNChecker(t *testing.T) {
	cli := loadSecrets(t)

	asnc := cli.ASNCheckerFor([]uint32{13335})

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

func BenchmarkWithCache(b *testing.B) {
	cli := loadSecrets(b)
	req := &iptoasnv1.LookupRequest{IpAddress: "1.1.1.1"}

	_, err := cli.IPToASN.Lookup(b.Context(), req)
	if err != nil {
		b.Error(err)
	}

	for b.Loop() {
		_, err := cli.IPToASN.Lookup(b.Context(), req)
		if err != nil {
			b.Error(err)
		}
	}
}

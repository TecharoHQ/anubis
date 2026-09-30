package lib

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/internal/dnsbl"
)

func TestDNSBLLookupDecision(t *testing.T) {
	for _, tc := range []struct {
		name    string
		verdict dnsbl.DroneBLResponse
		err     error
		deny    bool
		calls   int
	}{
		{"listed", dnsbl.HTTPProxy, nil, true, 1},
		{"clean", dnsbl.AllGood, nil, false, 1},
		{"transient error", dnsbl.Unknown, errors.New("lookup failed"), true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spawnAnubis(t, Options{})
			s.policy.DNSBL = true
			calls := 0
			lookup := func(string) (dnsbl.DroneBLResponse, error) { calls++; return tc.verdict, tc.err }
			for range 2 {
				r := httptest.NewRequest("GET", "http://example.com/", nil)
				if got := s.handleDNSBLWithLookup(httptest.NewRecorder(), r, "192.0.2.1", s.logger, lookup); got != tc.deny {
					t.Errorf("deny=%v want %v", got, tc.deny)
				}
			}
			if calls != tc.calls {
				t.Errorf("lookups=%d want %d", calls, tc.calls)
			}
		})
	}
}

func TestDNSBLRecoversAfterLookupError(t *testing.T) {
	s := spawnAnubis(t, Options{})
	s.policy.DNSBL = true
	calls := 0
	lookup := func(string) (dnsbl.DroneBLResponse, error) {
		calls++
		if calls == 1 {
			return dnsbl.Unknown, errors.New("lookup failed")
		}
		return dnsbl.AllGood, nil
	}
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	w := httptest.NewRecorder()
	if !s.handleDNSBLWithLookup(w, r, "192.0.2.2", s.logger, lookup) || w.Code != 503 {
		t.Fatalf("status=%d", w.Code)
	}
	if s.handleDNSBLWithLookup(httptest.NewRecorder(), r, "192.0.2.2", s.logger, lookup) {
		t.Fatal("recovered lookup denied")
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

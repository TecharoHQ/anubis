package fcrdns

import (
	"net/netip"
	"regexp"
	"testing"
	"time"
)

func TestFCrDNSCheck(t *testing.T) {
	localhostRegex := regexp.MustCompile("^localhost$")
	localhost := netip.MustParseAddr("127.0.0.1")
	localhostV6 := netip.MustParseAddr("::1")

	tests := []struct {
		name     string
		clientIp string
		host     string
		hostIp   netip.Addr
		regexp   *regexp.Regexp
		expected bool
	}{
		{
			name:     "IPv4",
			clientIp: "127.0.0.1",
			host:     "localhost",
			hostIp:   localhost,
			regexp:   localhostRegex,
			expected: true,
		},
		{
			name:     "IPv6",
			clientIp: "::1",
			host:     "localhost",
			hostIp:   localhostV6,
			regexp:   localhostRegex,
			expected: true,
		},
		{
			name:     "No regexp match",
			clientIp: "127.0.0.1",
			host:     "localhost",
			hostIp:   localhost,
			regexp:   regexp.MustCompile("^remotehost$"),
			expected: false,
		},
		{
			name:     "No reverse DNS record",
			clientIp: "127.0.0.1",
			regexp:   localhostRegex,
			expected: false,
		},
		{
			name:     "No forward DNS record",
			clientIp: "127.0.0.1",
			host:     "localhost",
			regexp:   localhostRegex,
			expected: false,
		},
		{
			name:     "IP mismatch",
			clientIp: "127.0.0.1",
			host:     "localhost",
			hostIp:   netip.IPv4Unspecified(),
			regexp:   localhostRegex,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFCrDNS()
			f.resolver = nil // There shouldn't be any network real requests for these tests.

			if tt.host != "" {
				f.reverseLookupCache.Set(tt.clientIp, []string{tt.host}, time.Hour)

				if !tt.hostIp.IsValid() {
					f.forwardLookupCacheV4.Set(tt.host, []netip.Addr{}, time.Hour)
					f.forwardLookupCacheV6.Set(tt.host, []netip.Addr{}, time.Hour)
				} else if tt.hostIp.Is6() {
					f.forwardLookupCacheV6.Set(tt.host, []netip.Addr{tt.hostIp}, time.Hour)
				} else {
					f.forwardLookupCacheV4.Set(tt.host, []netip.Addr{tt.hostIp}, time.Hour)
				}
			} else {
				f.reverseLookupCache.Set(tt.clientIp, []string{}, time.Hour)
			}

			if ok, err := f.Check(tt.clientIp, tt.regexp); err != nil {
				t.Errorf("unexpected error: %v", err)
			} else if ok != tt.expected {
				t.Errorf("expected: %t, got: %t", tt.expected, ok)
			}
		})
	}
}

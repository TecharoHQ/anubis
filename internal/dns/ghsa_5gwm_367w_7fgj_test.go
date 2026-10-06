package dns

import (
	"context"
	"net"
	"testing"

	"github.com/neilotoole/slogt/v2"
)

// TestGHSA_5gwm_367w_7fgj requires the same PTR hostname to match the
// policy pattern and forward-resolve to the original IP address.
func TestGHSA_5gwm_367w_7fgj(t *testing.T) {
	const addr = "192.0.2.42"
	pattern := `^.*\.yandex\.(ru|com|net)$`

	tests := []struct {
		name    string
		names   []string
		forward map[string][]string
		want    bool
	}{
		{
			name:    "matching PTR does not resolve",
			names:   []string{"fake.yandex.com.", "owned.example."},
			forward: map[string][]string{"owned.example": {addr}},
		},
		{
			name:    "nonmatching PTR comes first",
			names:   []string{"owned.example.", "fake.yandex.com."},
			forward: map[string][]string{"owned.example": {addr}},
		},
		{
			name:  "matching PTR resolves to another IP",
			names: []string{"fake.yandex.com.", "owned.example."},
			forward: map[string][]string{
				"fake.yandex.com": {"192.0.2.99"},
				"owned.example":   {addr},
			},
		},
		{
			name:    "forward confirmed hostname does not match",
			names:   []string{"owned.example."},
			forward: map[string][]string{"owned.example": {addr}},
		},
		{
			name:    "matching hostname is forward confirmed",
			names:   []string{"real.yandex.com."},
			forward: map[string][]string{"real.yandex.com": {addr}},
			want:    true,
		},
		{
			name:    "later matching hostname is forward confirmed",
			names:   []string{"fake.yandex.com.", "real.yandex.com."},
			forward: map[string][]string{"real.yandex.com": {addr}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalAddr, originalHost := DNSLookupAddr, DNSLookupHost
			t.Cleanup(func() {
				DNSLookupAddr, DNSLookupHost = originalAddr, originalHost
			})
			DNSLookupAddr = func(string) ([]string, error) {
				return tt.names, nil
			}
			DNSLookupHost = func(host string) ([]string, error) {
				if addrs, ok := tt.forward[host]; ok {
					return addrs, nil
				}
				return nil, &net.DNSError{Name: host, IsNotFound: true}
			}

			d := New(context.Background(), nil, slogt.New(t))
			if got := d.VerifyFCrDNS(addr, &pattern); got != tt.want {
				t.Errorf("VerifyFCrDNS(%q, %q) with PTRs %v = %v, want %v", addr, pattern, tt.names, got, tt.want)
			}
		})
	}
}

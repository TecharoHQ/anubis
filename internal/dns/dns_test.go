package dns

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/store/memory"
)

// newTestDNS is a helper function to create a new Dns object with an in-memory cache for testing.
func newTestDNS(forwardTTL int, reverseTTL int) *Dns {
	ctx := context.Background()
	memStore := memory.New(ctx)
	cache := NewDNSCache(forwardTTL, reverseTTL, memStore)
	return New(ctx, cache)
}

// mockLookupAddr is a mock implementation of the net.LookupAddr function.
func mockLookupAddr(addr string) ([]string, error) {
	switch addr {
	case "8.8.8.8":
		return []string{"dns.google."}, nil
	case "1.1.1.1":
		return []string{"one.one.one.one."}, nil
	case "208.67.222.222":
		return []string{"resolver1.opendns.com."}, nil
	case "9.9.9.9":
		return nil, &net.DNSError{Err: "no such host", Name: "9.9.9.9", IsNotFound: true}
	default:
		return nil, &net.DNSError{Err: "no such host", Name: addr, IsNotFound: true}
	}
}

// mockLookupHost is a mock implementation of the net.LookupHost function.
func mockLookupHost(host string) ([]string, error) {
	switch host {
	case "dns.google":
		return []string{"8.8.8.8", "8.8.4.4"}, nil
	case "one.one.one.one":
		return []string{"1.1.1.1", "1.0.0.1"}, nil
	case "resolver1.opendns.com":
		return []string{"208.67.222.222"}, nil
	case "example.com":
		return nil, &net.DNSError{Err: "no such host", Name: "example.com", IsNotFound: true}
	default:
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
}

func TestMain(m *testing.M) {
	// Before all tests
	originalLookupAddr := DNSLookupAddr
	originalLookupHost := DNSLookupHost

	DNSLookupAddr = mockLookupAddr
	DNSLookupHost = mockLookupHost

	// Run tests
	exitCode := m.Run()

	// After all tests
	DNSLookupAddr = originalLookupAddr
	DNSLookupHost = originalLookupHost

	// Exit
	if exitCode != 0 {
		panic(exitCode)
	}
}

func TestDns_ArpaReverseIP(t *testing.T) {
	d := newTestDNS(0, 0)
	tests := []struct {
		name    string
		ip      string
		want    string
		wantErr bool
	}{
		{"ipv4", "192.0.2.1", "1.2.0.192", false},
		{"ipv6", "2001:db8::1", "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2", false},
		{"invalid ip", "invalid", "invalid", true},
		{"ipv4-mapped ipv6", "::ffff:192.0.2.1", "1.2.0.192", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.ArpaReverseIP(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("ArpaReverseIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ArpaReverseIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDns_ReverseDNS(t *testing.T) {
	d := newTestDNS(1, 1) // short TTL for testing cache

	// First call - cache miss
	t.Run("cache miss", func(t *testing.T) {
		got, err := d.ReverseDNS("8.8.8.8")
		if err != nil {
			t.Fatalf("ReverseDNS() error = %v", err)
		}
		want := []string{"dns.google"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ReverseDNS() = %v, want %v", got, want)
		}
	})

	// Second call - cache hit
	t.Run("cache hit", func(t *testing.T) {
		// Temporarily replace lookup function to ensure cache is used
		originalLookupAddr := DNSLookupAddr
		DNSLookupAddr = func(addr string) ([]string, error) {
			return nil, errors.New("should not be called")
		}
		defer func() { DNSLookupAddr = originalLookupAddr }()

		got, err := d.ReverseDNS("8.8.8.8")
		if err != nil {
			t.Fatalf("ReverseDNS() error = %v", err)
		}
		want := []string{"dns.google"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ReverseDNS() = %v, want %v", got, want)
		}
	})

	// Test cache expiration
	t.Run("cache expiration", func(t *testing.T) {
		time.Sleep(2 * time.Second)
		// Now the cache should be expired
		// We expect the mock to be called again
		// To test this we will change the mock to return something different
		originalLookupAddr := DNSLookupAddr
		DNSLookupAddr = func(addr string) ([]string, error) {
			if addr == "8.8.8.8" {
				return []string{"expired.google."}, nil
			}
			return mockLookupAddr(addr)
		}
		defer func() { DNSLookupAddr = originalLookupAddr }()

		got, err := d.ReverseDNS("8.8.8.8")
		if err != nil {
			t.Fatalf("ReverseDNS() error = %v", err)
		}
		want := []string{"expired.google"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ReverseDNS() = %v, want %v", got, want)
		}
	})

	// Test not found
	t.Run("not found", func(t *testing.T) {
		got, err := d.ReverseDNS("9.9.9.9")
		if err != nil {
			t.Fatalf("ReverseDNS() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("ReverseDNS() = %v, want empty slice", got)
		}
	})
}

func TestDns_LookupHost(t *testing.T) {
	d := newTestDNS(1, 1)

	t.Run("cache miss", func(t *testing.T) {
		got, err := d.LookupHost("dns.google")
		if err != nil {
			t.Fatalf("LookupHost() error = %v", err)
		}
		want := []string{"8.8.8.8", "8.8.4.4"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LookupHost() = %v, want %v", got, want)
		}
	})

	t.Run("cache hit", func(t *testing.T) {
		originalLookupHost := DNSLookupHost
		DNSLookupHost = func(host string) ([]string, error) {
			return nil, errors.New("should not be called")
		}
		defer func() { DNSLookupHost = originalLookupHost }()

		got, err := d.LookupHost("dns.google")
		if err != nil {
			t.Fatalf("LookupHost() error = %v", err)
		}
		want := []string{"8.8.8.8", "8.8.4.4"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LookupHost() = %v, want %v", got, want)
		}
	})

	t.Run("cache expiration", func(t *testing.T) {
		time.Sleep(2 * time.Second)
		originalLookupHost := DNSLookupHost
		DNSLookupHost = func(host string) ([]string, error) {
			if host == "dns.google" {
				return []string{"9.9.9.9"}, nil
			}
			return mockLookupHost(host)
		}
		defer func() { DNSLookupHost = originalLookupHost }()

		got, err := d.LookupHost("dns.google")
		if err != nil {
			t.Fatalf("LookupHost() error = %v", err)
		}
		want := []string{"9.9.9.9"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LookupHost() = %v, want %v", got, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		got, err := d.LookupHost("example.com")
		if err != nil {
			t.Fatalf("LookupHost() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("LookupHost() = %v, want empty slice", got)
		}
	})
}

func TestDns_VerifyFCrDNS(t *testing.T) {
	d := newTestDNS(1, 1)

	// Helper to convert string to *string
	p := func(s string) *string {
		return &s
	}

	tests := []struct {
		name    string
		ip      string
		pattern *string
		want    bool
	}{
		// Cases without pattern
		{"valid no pattern", "8.8.8.8", nil, true},
		{"valid partial no pattern", "1.1.1.1", nil, true},
		{"invalid no pattern", "9.9.9.9", nil, false},
		{"reverse lookup fails no pattern", "1.2.3.4", nil, false},

		// Cases with pattern
		{"valid match", "8.8.8.8", p(`.*\.google$`), true},
		{"valid no match", "8.8.8.8", p(`\.com$`), false},
		{"reverse lookup fails with pattern", "9.9.9.9", p(".*"), false},
		{"invalid pattern", "8.8.8.8", p(`[`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := d.VerifyFCrDNS(tt.ip, tt.pattern); got != tt.want {
				t.Errorf("VerifyFCrDNS() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("reverse cache hit", func(t *testing.T) {
		// Prime the cache
		if got := d.VerifyFCrDNS("8.8.8.8", nil); got != true {
			t.Fatalf("VerifyFCrDNS() priming failed, got %v, want true", got)
		}

		// Now test with a failing lookup to ensure cache is used
		originalLookupAddr := DNSLookupAddr
		DNSLookupAddr = func(addr string) ([]string, error) {
			return nil, errors.New("should not be called")
		}
		defer func() { DNSLookupAddr = originalLookupAddr }()

		if got := d.VerifyFCrDNS("8.8.8.8", nil); got != true {
			t.Errorf("VerifyFCrDNS() = %v, want true", got)
		}
	})

	t.Run("forward cache hit", func(t *testing.T) {
		// Prime the cache
		if got := d.VerifyFCrDNS("8.8.8.8", nil); got != true {
			t.Fatalf("VerifyFCrDNS() priming failed, got %v, want true", got)
		}

		// Now test with a failing lookup to ensure cache is used
		originalLookupHost := DNSLookupHost
		DNSLookupHost = func(host string) ([]string, error) {
			return nil, errors.New("should not be called")
		}
		defer func() { DNSLookupHost = originalLookupHost }()

		if got := d.VerifyFCrDNS("8.8.8.8", nil); got != true {
			t.Errorf("VerifyFCrDNS() = %v, want true", got)
		}
	})
}

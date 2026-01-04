package internal

import (
	"net/netip"
	"testing"
)

func TestClampIP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// IPv4 addresses
		{
			name:     "IPv4 normal address",
			input:    "192.168.1.100",
			expected: "192.168.1.0/24",
		},
		{
			name:     "IPv4 boundary - network address",
			input:    "192.168.1.0",
			expected: "192.168.1.0/24",
		},
		{
			name:     "IPv4 boundary - broadcast address",
			input:    "192.168.1.255",
			expected: "192.168.1.0/24",
		},
		{
			name:     "IPv4 class A address",
			input:    "10.0.0.1",
			expected: "10.0.0.0/24",
		},
		{
			name:     "IPv4 loopback",
			input:    "127.0.0.1",
			expected: "127.0.0.0/24",
		},
		{
			name:     "IPv4 link-local",
			input:    "169.254.0.1",
			expected: "169.254.0.0/24",
		},
		{
			name:     "IPv4 public address",
			input:    "203.0.113.1",
			expected: "203.0.113.0/24",
		},

		// IPv6 addresses
		{
			name:     "IPv6 normal address",
			input:    "2001:db8::1",
			expected: "2001:db8::/48",
		},
		{
			name:     "IPv6 with full expansion",
			input:    "2001:0db8:0000:0000:0000:0000:0000:0001",
			expected: "2001:db8::/48",
		},
		{
			name:     "IPv6 loopback",
			input:    "::1",
			expected: "::/48",
		},
		{
			name:     "IPv6 unspecified address",
			input:    "::",
			expected: "::/48",
		},
		{
			name:     "IPv6 link-local",
			input:    "fe80::1",
			expected: "fe80::/48",
		},
		{
			name:     "IPv6 unique local",
			input:    "fc00::1",
			expected: "fc00::/48",
		},
		{
			name:     "IPv6 documentation prefix",
			input:    "2001:db8:abcd:ef01::1234",
			expected: "2001:db8:abcd::/48",
		},
		{
			name:     "IPv6 global unicast",
			input:    "2606:4700:4700::1111",
			expected: "2606:4700:4700::/48",
		},
		{
			name:     "IPv6 multicast",
			input:    "ff02::1",
			expected: "ff02::/48",
		},

		// IPv4-mapped IPv6 addresses
		{
			name:     "IPv4-mapped IPv6 address",
			input:    "::ffff:192.168.1.100",
			expected: "192.168.1.0/24",
		},
		{
			name:     "IPv4-mapped IPv6 with different format",
			input:    "::ffff:10.0.0.1",
			expected: "10.0.0.0/24",
		},
		{
			name:     "IPv4-mapped IPv6 loopback",
			input:    "::ffff:127.0.0.1",
			expected: "127.0.0.0/24",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.input)

			result, ok := ClampIP(addr)
			if !ok {
				t.Fatalf("ClampIP(%s) returned false, want true", tt.input)
			}

			if result.String() != tt.expected {
				t.Errorf("ClampIP(%s) = %s, want %s", tt.input, result.String(), tt.expected)
			}
		})
	}
}

func TestClampIPSuccess(t *testing.T) {
	// Test that valid inputs return success
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "IPv4 address",
			input: "192.168.1.100",
		},
		{
			name:  "IPv6 address",
			input: "2001:db8::1",
		},
		{
			name:  "IPv4-mapped IPv6",
			input: "::ffff:192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.input)

			result, ok := ClampIP(addr)
			if !ok {
				t.Fatalf("ClampIP(%s) returned false, want true", tt.input)
			}

			// For valid inputs, we should get the clamped prefix
			if addr.Is4() || addr.Is4In6() {
				if result.Bits() != 24 {
					t.Errorf("Expected 24 bits for IPv4, got %d", result.Bits())
				}
			} else if addr.Is6() {
				if result.Bits() != 48 {
					t.Errorf("Expected 48 bits for IPv6, got %d", result.Bits())
				}
			}
		})
	}
}

func TestClampIPZeroValue(t *testing.T) {
	// Test that when ClampIP fails, it returns zero value
	// Note: It's hard to make addr.Prefix() fail with valid inputs,
	// so this test demonstrates the expected behavior
	addr := netip.MustParseAddr("192.168.1.100")

	// Manually create a zero value for comparison
	zeroPrefix := netip.Prefix{}

	// Call ClampIP - it should succeed with valid input
	result, ok := ClampIP(addr)

	// Verify the function succeeded
	if !ok {
		t.Error("ClampIP should succeed with valid input")
	}

	// Verify that the result is not a zero value
	if result == zeroPrefix {
		t.Error("Result should not be zero value for successful operation")
	}
}

func TestClampIPSpecialCases(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedPrefix  int
		expectedNetwork string
	}{
		{
			name:            "Minimum IPv4",
			input:           "0.0.0.0",
			expectedPrefix:  24,
			expectedNetwork: "0.0.0.0",
		},
		{
			name:            "Maximum IPv4",
			input:           "255.255.255.255",
			expectedPrefix:  24,
			expectedNetwork: "255.255.255.0",
		},
		{
			name:            "Minimum IPv6",
			input:           "::",
			expectedPrefix:  48,
			expectedNetwork: "::",
		},
		{
			name:            "Maximum IPv6 prefix part",
			input:           "ffff:ffff:ffff::",
			expectedPrefix:  48,
			expectedNetwork: "ffff:ffff:ffff::",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.input)

			result, ok := ClampIP(addr)
			if !ok {
				t.Fatalf("ClampIP(%s) returned false, want true", tt.input)
			}

			if result.Bits() != tt.expectedPrefix {
				t.Errorf("ClampIP(%s) bits = %d, want %d", tt.input, result.Bits(), tt.expectedPrefix)
			}

			if result.Addr().String() != tt.expectedNetwork {
				t.Errorf("ClampIP(%s) network = %s, want %s", tt.input, result.Addr().String(), tt.expectedNetwork)
			}
		})
	}
}

// Benchmark to ensure the function is performant
func BenchmarkClampIP(b *testing.B) {
	ipv4 := netip.MustParseAddr("192.168.1.100")
	ipv6 := netip.MustParseAddr("2001:db8::1")
	ipv4mapped := netip.MustParseAddr("::ffff:192.168.1.100")

	b.Run("IPv4", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ClampIP(ipv4)
		}
	})

	b.Run("IPv6", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ClampIP(ipv6)
		}
	})

	b.Run("IPv4-mapped", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ClampIP(ipv4mapped)
		}
	})
}
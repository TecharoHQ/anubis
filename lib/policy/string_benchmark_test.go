package policy

import (
	"fmt"
	"strings"
	"testing"
)

// Test data - typical CIDR ranges that might be used
var testCIDRsForString = []string{
	"192.168.0.0/16",
	"10.0.0.0/8", 
	"172.16.0.0/12",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"203.0.113.0/24",
	"198.18.0.0/15",
	"100.64.0.0/10",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"2001:db8::/32",
	"fe80::/10",
	"::1/128",
	"2000::/3",
	"fc00::/7",
	"ff00::/8",
}

// BenchmarkStringBuilding_Fprintln benchmarks using fmt.Fprintln
func BenchmarkStringBuilding_Fprintln(b *testing.B) {
	cidrs := testCIDRsForString
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		for _, cidr := range cidrs {
			fmt.Fprintln(&sb, cidr)
		}
		_ = sb.String()
	}
}

// BenchmarkStringBuilding_WriteString benchmarks using WriteString + WriteByte
func BenchmarkStringBuilding_WriteString(b *testing.B) {
	cidrs := testCIDRsForString
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		for i, cidr := range cidrs {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(cidr)
		}
		_ = sb.String()
	}
}

// BenchmarkStringBuilding_Join benchmarks using strings.Join (for comparison)
func BenchmarkStringBuilding_Join(b *testing.B) {
	cidrs := testCIDRsForString
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = strings.Join(cidrs, "\n")
	}
}

// BenchmarkStringBuilding_FprintlnSmall benchmarks with fewer CIDRs (common case)
func BenchmarkStringBuilding_FprintlnSmall(b *testing.B) {
	cidrs := testCIDRsForString[:3] // Just first 3 CIDRs
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		for _, cidr := range cidrs {
			fmt.Fprintln(&sb, cidr)
		}
		_ = sb.String()
	}
}

// BenchmarkStringBuilding_WriteStringSmall benchmarks with fewer CIDRs (common case)
func BenchmarkStringBuilding_WriteStringSmall(b *testing.B) {
	cidrs := testCIDRsForString[:3] // Just first 3 CIDRs
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		for i, cidr := range cidrs {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(cidr)
		}
		_ = sb.String()
	}
}
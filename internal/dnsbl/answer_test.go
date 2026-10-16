package dnsbl

import (
	"net"
	"testing"
)

func TestNonIPv4Answers(t *testing.T) {
	for _, tc := range []struct {
		name string
		ips  []net.IP
		want DroneBLResponse
	}{
		{"ipv6 only", []net.IP{net.ParseIP("::1")}, UnknownSpambotOrDrone},
		{"mixed", []net.IP{net.ParseIP("::1"), net.ParseIP("127.0.0.9")}, HTTPProxy},
		{"ipv4", []net.IP{net.ParseIP("127.0.0.3")}, IRCDrone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := responseFromIPs(tc.ips); got != tc.want {
				t.Fatalf("got=%v want=%v", got, tc.want)
			}
		})
	}
}

package thoth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"

	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
	"github.com/gaissmai/bart"
	"google.golang.org/grpc"
)

type IPToASNWithCache struct {
	next  iptoasnv1.IpToASNServiceClient
	table *bart.Table[*iptoasnv1.LookupResponse]
}

func NewIpToASNWithCache(next iptoasnv1.IpToASNServiceClient) *IPToASNWithCache {
	result := &IPToASNWithCache{
		next:  next,
		table: &bart.Table[*iptoasnv1.LookupResponse]{},
	}

	for _, pfx := range []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),         // RFC 1918
		netip.MustParsePrefix("172.16.0.0/12"),      // RFC 1918
		netip.MustParsePrefix("192.168.0.0/16"),     // RFC 1918
		netip.MustParsePrefix("127.0.0.0/8"),        // Loopback
		netip.MustParsePrefix("169.254.0.0/16"),     // Link-local
		netip.MustParsePrefix("100.64.0.0/10"),      // CGNAT
		netip.MustParsePrefix("192.0.0.0/24"),       // Protocol assignments
		netip.MustParsePrefix("192.0.2.0/24"),       // TEST-NET-1
		netip.MustParsePrefix("198.18.0.0/15"),      // Benchmarking
		netip.MustParsePrefix("198.51.100.0/24"),    // TEST-NET-2
		netip.MustParsePrefix("203.0.113.0/24"),     // TEST-NET-3
		netip.MustParsePrefix("240.0.0.0/4"),        // Reserved
		netip.MustParsePrefix("255.255.255.255/32"), // Broadcast
		netip.MustParsePrefix("fc00::/7"),           // Unique local address
		netip.MustParsePrefix("fe80::/10"),          // Link-local
		netip.MustParsePrefix("::1/128"),            // Loopback
		netip.MustParsePrefix("::/128"),             // Unspecified
		netip.MustParsePrefix("100::/64"),           // Discard-only
		netip.MustParsePrefix("2001:db8::/32"),      // Documentation
	} {
		result.table.Insert(pfx, &iptoasnv1.LookupResponse{Announced: false})
	}

	return result
}

func (ip2asn *IPToASNWithCache) Lookup(ctx context.Context, lr *iptoasnv1.LookupRequest, opts ...grpc.CallOption) (*iptoasnv1.LookupResponse, error) {
	addr, err := netip.ParseAddr(lr.GetIpAddress())
	if err != nil {
		return nil, fmt.Errorf("input is not an IP address: %w", err)
	}

	cachedResponse, ok := ip2asn.table.Lookup(addr)
	if ok {
		return cachedResponse, nil
	}

	resp, err := ip2asn.next.Lookup(ctx, lr, opts...)
	if err != nil {
		return nil, err
	}

	var errs []error
	for _, cidr := range resp.GetCidr() {
		pfx, err := netip.ParsePrefix(cidr)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ip2asn.table.Insert(pfx, resp)
	}

	if len(errs) != 0 {
		slog.Error("errors parsing IP prefixes", "err", errors.Join(errs...))
	}

	return resp, nil
}

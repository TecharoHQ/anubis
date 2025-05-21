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
	return &IPToASNWithCache{
		next:  next,
		table: &bart.Table[*iptoasnv1.LookupResponse]{},
	}
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

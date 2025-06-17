package thothmock

import (
	"context"
	"net/netip"

	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func MockIpToASNService() *IpToASNService {
	responses := map[string]*iptoasnv1.LookupResponse{
		"127.0.0.1": {Announced: false},
		"::1":       {Announced: false},
		"10.10.10.10": {
			Announced:   true,
			AsNumber:    13335,
			Cidr:        []string{"1.1.1.0/24"},
			CountryCode: "US",
			Description: "Cloudflare",
		},
		"2.2.2.2": {
			Announced:   true,
			AsNumber:    420,
			Cidr:        []string{"2.2.2.0/24"},
			CountryCode: "CA",
			Description: "test canada",
		},
		"1.1.1.1": {
			Announced:   true,
			AsNumber:    13335,
			Cidr:        []string{"1.1.1.0/24"},
			CountryCode: "US",
			Description: "Cloudflare",
		},
	}

	return &IpToASNService{Responses: responses}
}

type IpToASNService struct {
	iptoasnv1.UnimplementedIpToASNServiceServer
	Responses map[string]*iptoasnv1.LookupResponse
}

func (ip2asn *IpToASNService) Lookup(ctx context.Context, lr *iptoasnv1.LookupRequest, opts ...grpc.CallOption) (*iptoasnv1.LookupResponse, error) {
	if _, err := netip.ParseAddr(lr.GetIpAddress()); err != nil {
		return nil, err
	}

	resp, ok := ip2asn.Responses[lr.GetIpAddress()]
	if !ok {
		return nil, status.Error(codes.NotFound, "IP address not found in mock")
	}

	return resp, nil
}

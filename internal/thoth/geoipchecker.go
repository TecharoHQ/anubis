package thoth

import (
	"context"
	"net/http"
	"strings"
	"time"

	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
)

type GeoIPChecker struct {
	iptoasn   iptoasnv1.IpToASNServiceClient
	countries map[string]struct{}
	hash      string
}

func (gipc *GeoIPChecker) Check(r *http.Request) (bool, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Millisecond)
	defer cancel()

	ipInfo, err := gipc.iptoasn.Lookup(ctx, &iptoasnv1.LookupRequest{
		IpAddress: r.Header.Get("X-Real-Ip"),
	})
	if err != nil {
		return false, err
	}

	_, ok := gipc.countries[strings.ToLower(ipInfo.GetCountryCode())]

	return ok, nil
}

func (gipc *GeoIPChecker) Hash() string {
	return gipc.hash
}

package thoth

import (
	"context"
	"net/http"
	"time"

	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
)

type ASNChecker struct {
	iptoasn iptoasnv1.IpToASNServiceClient
	asns    map[int]struct{}
	hash    string
}

func (asnc *ASNChecker) Check(r *http.Request) (bool, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 50*time.Millisecond)
	defer cancel()

	ipInfo, err := asnc.iptoasn.Lookup(ctx, &iptoasnv1.LookupRequest{
		IpAddress: r.Header.Get("X-Real-Ip"),
	})
	if err != nil {
		return false, err
	}

	_, ok := asnc.asns[int(ipInfo.GetAsNumber())]

	return ok, nil
}

func (asnc *ASNChecker) Hash() string {
	return asnc.hash
}

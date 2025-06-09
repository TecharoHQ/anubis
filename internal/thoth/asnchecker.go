package thoth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
)

type ASNChecker struct {
	iptoasn iptoasnv1.IpToASNServiceClient
	asns    map[uint32]struct{}
	hash    string
}

func (asnc *ASNChecker) Check(r *http.Request) (bool, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	ipInfo, err := asnc.iptoasn.Lookup(ctx, &iptoasnv1.LookupRequest{
		IpAddress: r.Header.Get("X-Real-Ip"),
	})
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			slog.Debug("error contacting thoth", "err", err, "actionable", false)
			return false, nil
		default:
			return false, err
		}
	}

	// If IP is not publicly announced, return false
	if !ipInfo.GetAnnounced() {
		return false, nil
	}

	_, ok := asnc.asns[uint32(ipInfo.GetAsNumber())]

	return ok, nil
}

func (asnc *ASNChecker) Hash() string {
	return asnc.hash
}

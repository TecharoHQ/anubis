package thoth

import (
	"context"
	"errors"
	"log/slog"
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
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	ipInfo, err := gipc.iptoasn.Lookup(ctx, &iptoasnv1.LookupRequest{
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

	_, ok := gipc.countries[strings.ToLower(ipInfo.GetCountryCode())]

	return ok, nil
}

func (gipc *GeoIPChecker) Hash() string {
	return gipc.hash
}

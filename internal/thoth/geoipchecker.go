package thoth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/lib/policy/checker"
	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
)

func (c *Client) GeoIPCheckerFor(countries []string) checker.Impl {
	countryMap := map[string]struct{}{}
	var sb strings.Builder
	fmt.Fprintln(&sb, "GeoIPChecker")
	for _, cc := range countries {
		countryMap[cc] = struct{}{}
		fmt.Fprintln(&sb, cc)
	}

	return &GeoIPChecker{
		IPToASN:   c.IPToASN,
		Countries: countryMap,
		hash:      sb.String(),
	}
}

type GeoIPChecker struct {
	IPToASN   iptoasnv1.IpToASNServiceClient
	Countries map[string]struct{}
	hash      string
}

func (gipc *GeoIPChecker) Check(r *http.Request) (bool, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	ipInfo, err := gipc.IPToASN.Lookup(ctx, &iptoasnv1.LookupRequest{
		IpAddress: r.Header.Get("X-Real-Ip"),
	})
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			slog.Debug("error contacting thoth", "err", err, "actionable", false)
			return false, nil
		default:
			slog.Error("error contacting thoth, please contact support", "err", err, "actionable", true)
			return false, nil
		}
	}

	// If IP is not publicly announced, return false
	if !ipInfo.GetAnnounced() {
		return false, nil
	}

	_, ok := gipc.Countries[strings.ToLower(ipInfo.GetCountryCode())]

	return ok, nil
}

func (gipc *GeoIPChecker) Hash() string {
	return gipc.hash
}

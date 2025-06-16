package thoth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	iptoasnv1 "github.com/TecharoHQ/thoth-proto/gen/techaro/thoth/iptoasn/v1"
)

func (c *Client) ASNCheckerFor(asns []uint32) checker.Impl {
	asnMap := map[uint32]struct{}{}
	var sb strings.Builder
	fmt.Fprintln(&sb, "ASNChecker")
	for _, asn := range asns {
		asnMap[asn] = struct{}{}
		fmt.Fprintln(&sb, "AS", asn)
	}

	return &ASNChecker{
		iptoasn: c.IPToASN,
		asns:    asnMap,
		hash:    internal.SHA256sum(sb.String()),
	}
}

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
			slog.Error("error contacting thoth, please contact support", "err", err, "actionable", true)
			return false, nil
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

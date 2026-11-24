package systemone

import (
	"log/slog"
	"net/http"
	"net/netip"

	"github.com/TecharoHQ/anubis/lib/geoip"
)

type geoIPData struct {
	ASN         uint32
	ISP         string
	CountryCode string
}

func (gipd geoIPData) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Uint64("asn", uint64(gipd.ASN)),
		slog.String("isp", gipd.ISP),
		slog.String("country", gipd.CountryCode),
	)
}

// lookupGeoIP resolves the ASN for the request's client IP using the GeoIP
// database threaded through the request context by the Anubis server.
func lookupGeoIP(r *http.Request) (gipd *geoIPData, ok bool) {
	db, ok := geoip.FromContext(r.Context())
	if !ok {
		return nil, false
	}

	addr, err := netip.ParseAddr(r.Header.Get("X-Real-IP"))
	if err != nil {
		return nil, false
	}

	asn, isp, ok := db.LookupASN(addr)
	if !ok {
		return nil, false
	}

	cc, ok := db.LookupCountry(addr)
	if !ok {
		return nil, false
	}

	return &geoIPData{
		ASN:         asn,
		ISP:         isp,
		CountryCode: cc,
	}, true
}

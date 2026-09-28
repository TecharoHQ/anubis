package geoip

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
)

// ASNCheckerFor matches requests from any of the given autonomous systems.
func (db *DB) ASNCheckerFor(asns []uint32) checker.Impl {
	asnMap := map[uint32]struct{}{}
	var sb strings.Builder
	fmt.Fprintln(&sb, "ASNChecker")
	for _, asn := range asns {
		asnMap[asn] = struct{}{}
		fmt.Fprintln(&sb, "AS", asn)
	}

	return &ASNChecker{
		db:   db,
		asns: asnMap,
		hash: internal.FastHash(sb.String()),
	}
}

type ASNChecker struct {
	db   *DB
	asns map[uint32]struct{}
	hash string
}

func (asnc *ASNChecker) Check(r *http.Request) (bool, error) {
	addr, err := netip.ParseAddr(r.Header.Get("X-Real-IP"))
	if err != nil {
		return false, nil
	}

	asn, _, ok := asnc.db.LookupASN(addr)
	if !ok {
		return false, nil
	}

	_, ok = asnc.asns[asn]
	return ok, nil
}

func (asnc *ASNChecker) Hash() string {
	return asnc.hash
}

// GeoIPCheckerFor matches requests from any of the given countries.
func (db *DB) GeoIPCheckerFor(countries []string) checker.Impl {
	countryMap := map[string]struct{}{}
	var sb strings.Builder
	fmt.Fprintln(&sb, "GeoIPChecker")
	for _, cc := range countries {
		countryMap[strings.ToLower(cc)] = struct{}{}
		fmt.Fprintln(&sb, cc)
	}

	return &GeoIPChecker{
		db:        db,
		countries: countryMap,
		hash:      sb.String(),
	}
}

type GeoIPChecker struct {
	db        *DB
	countries map[string]struct{}
	hash      string
}

func (gipc *GeoIPChecker) Check(r *http.Request) (bool, error) {
	addr, err := netip.ParseAddr(r.Header.Get("X-Real-IP"))
	if err != nil {
		return false, nil
	}

	cc, ok := gipc.db.LookupCountry(addr)
	if !ok {
		return false, nil
	}

	_, ok = gipc.countries[cc]
	return ok, nil
}

func (gipc *GeoIPChecker) Hash() string {
	return gipc.hash
}

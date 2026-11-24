package lib

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/geoip"
	"github.com/TecharoHQ/anubis/lib/geoip/geoiptest"
)

func TestGetRequestLoggerASN(t *testing.T) {
	for _, tt := range []struct {
		name     string
		logASN   bool
		withDB   bool
		ip       string
		wantASN  string
		wantDesc string
	}{
		{name: "announced", logASN: true, withDB: true, ip: "1.1.1.1", wantASN: "13335", wantDesc: "Cloudflare, Inc."},
		{name: "private", logASN: true, withDB: true, ip: "10.0.0.1"},
		{name: "not an IP", logASN: true, withDB: true, ip: "taco"},
		{name: "logging disabled", withDB: true, ip: "1.1.1.1"},
		{name: "no database", logASN: true, ip: "1.1.1.1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pol := loadPolicies(t, "", 4)
			pol.LogASN = tt.logASN
			pol.GeoIP = nil
			if tt.withDB {
				pol.GeoIP = geoiptest.DB(t)
			}

			srv := spawnAnubis(t, Options{Policy: pol})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-Real-IP", tt.ip)

			_, req = srv.getRequestLogger(req)
			asn, desc := asnFromContext(req.Context())
			if asn != tt.wantASN || desc != tt.wantDesc {
				t.Errorf("want (%q, %q), got (%q, %q)", tt.wantASN, tt.wantDesc, asn, desc)
			}
		})
	}
}

// TestGetRequestLoggerThreadsGeoIP ensures extensions can reach the GeoIP
// database from the request context in their Head and Validate methods.
func TestGetRequestLoggerThreadsGeoIP(t *testing.T) {
	pol := loadPolicies(t, "", 4)
	db := geoiptest.DB(t)
	pol.GeoIP = db

	srv := spawnAnubis(t, Options{Policy: pol})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, req = srv.getRequestLogger(req)

	got, ok := geoip.FromContext(req.Context())
	if !ok || got != db {
		t.Fatalf("geoip database not threaded through request context: ok=%v got=%p want=%p", ok, got, db)
	}
}

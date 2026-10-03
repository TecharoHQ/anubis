// Package geoiptest builds small MaxMind databases for tests.
package geoiptest

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/geoip"
	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/oschwald/maxminddb-golang/v2"
)

// Record is one network in a fixture database.
type Record struct {
	CIDR              string
	ASN               uint32
	Organization      string
	Country           string
	RegisteredCountry string
}

// DefaultRecords are the networks in the default fixtures. 10.0.0.0/8 and
// other reserved ranges are left out, like in the real databases.
var DefaultRecords = []Record{
	{CIDR: "1.1.1.0/24", ASN: 13335, Organization: "Cloudflare, Inc.", Country: "US", RegisteredCountry: "US"},
	{CIDR: "1.0.0.0/24", ASN: 13335, Organization: "Cloudflare, Inc.", RegisteredCountry: "AU"},
	{CIDR: "2.2.2.0/24", ASN: 420, Organization: "test canada", Country: "CA", RegisteredCountry: "CA"},
	{CIDR: "57.141.0.0/24", ASN: 32934, Organization: "Facebook, Inc.", Country: "US", RegisteredCountry: "IE"},
	{CIDR: "2606:4700::/32", ASN: 13335, Organization: "Cloudflare, Inc.", Country: "US", RegisteredCountry: "US"},
}

// ASNDatabase returns the bytes of an ASN database holding records.
func ASNDatabase(t testing.TB, records []Record) []byte {
	t.Helper()

	return build(t, "GeoLite2-ASN", records, func(r Record) mmdbtype.Map {
		return mmdbtype.Map{
			"autonomous_system_number":       mmdbtype.Uint32(r.ASN),
			"autonomous_system_organization": mmdbtype.String(r.Organization),
		}
	})
}

// CountryDatabase returns the bytes of a country database holding records.
func CountryDatabase(t testing.TB, records []Record) []byte {
	t.Helper()

	return build(t, "GeoLite2-Country", records, func(r Record) mmdbtype.Map {
		result := mmdbtype.Map{}
		if r.Country != "" {
			result["country"] = mmdbtype.Map{"iso_code": mmdbtype.String(r.Country)}
		}
		if r.RegisteredCountry != "" {
			result["registered_country"] = mmdbtype.Map{"iso_code": mmdbtype.String(r.RegisteredCountry)}
		}
		return result
	})
}

// WriteFixtures writes the default ASN and country databases into a temporary
// directory and returns their paths.
func WriteFixtures(t testing.TB) (asnPath, countryPath string) {
	t.Helper()

	dir := t.TempDir()
	asnPath = filepath.Join(dir, "GeoLite2-ASN.mmdb")
	countryPath = filepath.Join(dir, "GeoLite2-Country.mmdb")

	if err := os.WriteFile(asnPath, ASNDatabase(t, DefaultRecords), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(countryPath, CountryDatabase(t, DefaultRecords), 0o644); err != nil {
		t.Fatal(err)
	}

	return asnPath, countryPath
}

// DB returns a DB backed by the default fixtures.
func DB(t testing.TB) *geoip.DB {
	t.Helper()

	asn, err := maxminddb.OpenBytes(ASNDatabase(t, DefaultRecords))
	if err != nil {
		t.Fatal(err)
	}

	country, err := maxminddb.OpenBytes(CountryDatabase(t, DefaultRecords))
	if err != nil {
		t.Fatal(err)
	}

	return geoip.FromReaders(asn, country, config.DefaultGeoIPCountryField)
}

// WithMockGeoIP returns a context carrying a DB backed by the default fixtures.
func WithMockGeoIP(t testing.TB) context.Context {
	t.Helper()

	return geoip.With(t.Context(), DB(t))
}

func build(t testing.TB, dbType string, records []Record, value func(Record) mmdbtype.Map) []byte {
	t.Helper()

	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType: dbType,
		RecordSize:   24,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range records {
		_, network, err := net.ParseCIDR(r.CIDR)
		if err != nil {
			t.Fatal(err)
		}

		if err := tree.Insert(network, value(r)); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	if _, err := tree.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

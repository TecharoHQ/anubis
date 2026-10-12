package geoip_test

import (
	"bytes"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/geoip"
	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/oschwald/maxminddb-golang/v2"
)

func TestAdditionalMaxMindEditions(t *testing.T) {
	for _, tt := range []struct {
		name   string
		dbType string
		kind   string
		record mmdbtype.Map
	}{
		{
			name:   "ISP for ASN",
			dbType: "GeoIP2-ISP",
			kind:   "asn",
			record: mmdbtype.Map{
				"autonomous_system_number":       mmdbtype.Uint32(13335),
				"autonomous_system_organization": mmdbtype.String("Cloudflare"),
				"isp":                            mmdbtype.String("Cloudflare"),
			},
		},
		{
			name:   "City for Country",
			dbType: "GeoLite2-City",
			kind:   "country",
			record: mmdbtype.Map{
				"country": mmdbtype.Map{
					"iso_code": mmdbtype.String("US"),
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tree, err := mmdbwriter.New(mmdbwriter.Options{
				DatabaseType: tt.dbType,
				RecordSize:   24,
			})
			if err != nil {
				t.Fatal(err)
			}

			_, network, _ := net.ParseCIDR("1.1.1.0/24")
			if err := tree.Insert(network, tt.record); err != nil {
				t.Fatal(err)
			}

			var data bytes.Buffer
			if _, err := tree.WriteTo(&data); err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(t.TempDir(), "test.mmdb")
			if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}

			cfg := &config.GeoIPDatabases{}
			if tt.kind == "asn" {
				cfg.ASN = &config.GeoIPDatabase{Path: path}
			} else {
				cfg.Country = &config.GeoIPDatabase{Path: path}
			}

			// Verify that the normal Anubis loader accepts this edition.
			db, err := geoip.New(t.Context(), discardLogger(), cfg)
			if err != nil {
				t.Fatalf("failed to load %s: %v", tt.dbType, err)
			}

			ip := netip.MustParseAddr("1.1.1.1")

			if tt.kind == "asn" {
				asn, org, ok := db.LookupASN(ip)
				if !ok || asn != 13335 || org != "Cloudflare" {
					t.Fatalf("ASN lookup failed: %d %q %v", asn, org, ok)
				}
			} else {
				country, ok := db.LookupCountry(ip)
				if !ok || country != "us" {
					t.Fatalf("country lookup failed: %q %v", country, ok)
				}
			}

			t.Logf("PASS: %s loaded and lookup succeeded", tt.dbType)
		})
	}
}

// TestAdditionalMaxMindEdgeCases verifies that the existing lookup
// functions handle compatible database records correctly.
func TestAdditionalMaxMindEdgeCases(t *testing.T) {
	for _, tt := range []struct {
		name      string
		ip        string
		asn       uint32
		org       string
		country   string
		asnOK     bool
		countryOK bool
	}{
		{
			name:      "IPv4 lookup",
			ip:        "1.1.1.1",
			asn:       13335,
			org:       "Cloudflare",
			country:   "us",
			asnOK:     true,
			countryOK: true,
		},
		{
			name:      "IPv6 lookup",
			ip:        "2606:4700::1111",
			asn:       13335,
			org:       "Cloudflare",
			country:   "us",
			asnOK:     true,
			countryOK: true,
		},
		{
			name: "IP not in database",
			ip:   "8.8.8.8",
		},
		{
			name: "private IP",
			ip:   "10.0.0.1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			asnDB := createCompatibilityDB(t, "GeoIP2-ISP", true)
			countryDB := createCompatibilityDB(t, "GeoLite2-City", false)

			db := geoip.FromReaders(
				asnDB,
				countryDB,
				config.DefaultGeoIPCountryField,
			)

			ip := netip.MustParseAddr(tt.ip)

			asn, org, asnOK := db.LookupASN(ip)
			country, countryOK := db.LookupCountry(ip)

			if asn != tt.asn || org != tt.org || asnOK != tt.asnOK {
				t.Errorf(
					"ASN: wanted (%d, %q, %v), got (%d, %q, %v)",
					tt.asn, tt.org, tt.asnOK,
					asn, org, asnOK,
				)
			}

			if country != tt.country || countryOK != tt.countryOK {
				t.Errorf(
					"Country: wanted (%q, %v), got (%q, %v)",
					tt.country, tt.countryOK,
					country, countryOK,
				)
			}
		})
	}
}

func createCompatibilityDB(
	t *testing.T,
	dbType string,
	isASN bool,
) *maxminddb.Reader {
	t.Helper()

	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType: dbType,
		RecordSize:   24,
	})
	if err != nil {
		t.Fatal(err)
	}

	var record mmdbtype.Map

	if isASN {
		record = mmdbtype.Map{
			"autonomous_system_number":       mmdbtype.Uint32(13335),
			"autonomous_system_organization": mmdbtype.String("Cloudflare"),
		}
	} else {
		record = mmdbtype.Map{
			"country": mmdbtype.Map{
				"iso_code": mmdbtype.String("US"),
			},
		}
	}

	for _, cidr := range []string{
		"1.1.1.0/24",
		"2606:4700::/32",
	} {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatal(err)
		}

		if err := tree.Insert(network, record); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	if _, err := tree.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	reader, err := maxminddb.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		reader.Close()
	})

	return reader
}

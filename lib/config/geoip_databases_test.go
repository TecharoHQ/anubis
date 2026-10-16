package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testLicenseKeyEnv = "ANUBIS_TEST_MAXMIND_LICENSE_KEY"

func TestGeoIPDatabasesValid(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  *GeoIPDatabases
		setEnv bool
		errs   []error
	}{
		{
			name: "asn only",
			input: &GeoIPDatabases{
				ASN: &GeoIPDatabase{Path: "/tmp/asn.mmdb"},
			},
		},
		{
			name: "country only with registered_country",
			input: &GeoIPDatabases{
				Country: &GeoIPDatabase{
					Path:         "/tmp/country.mmdb",
					CountryField: new(CountryFieldRegisteredCountry),
				},
			},
		},
		{
			name: "both with auto update",
			input: &GeoIPDatabases{
				ASN:     &GeoIPDatabase{Path: "/tmp/asn.mmdb"},
				Country: &GeoIPDatabase{Path: "/tmp/country.mmdb"},
				AutoUpdate: &GeoIPAutoUpdate{
					AccountID:     1234,
					LicenseKeyEnv: testLicenseKeyEnv,
					Interval:      new("72h"),
					Endpoint:      new("https://updates.maxmind.com"),
				},
			},
			setEnv: true,
		},
		{
			name:  "no databases",
			input: &GeoIPDatabases{},
			errs:  []error{ErrGeoIPNoDatabases},
		},
		{
			name: "no path",
			input: &GeoIPDatabases{
				ASN:     &GeoIPDatabase{},
				Country: &GeoIPDatabase{},
			},
			errs: []error{ErrGeoIPNoPath},
		},
		{
			name: "empty edition id",
			input: &GeoIPDatabases{
				ASN: &GeoIPDatabase{Path: "/tmp/asn.mmdb", EditionID: new("")},
			},
			errs: []error{ErrGeoIPEmptyEditionID},
		},
		{
			name: "country field on asn",
			input: &GeoIPDatabases{
				ASN: &GeoIPDatabase{Path: "/tmp/asn.mmdb", CountryField: new(CountryFieldCountry)},
			},
			errs: []error{ErrGeoIPCountryFieldOnASN},
		},
		{
			name: "invalid country field",
			input: &GeoIPDatabases{
				Country: &GeoIPDatabase{Path: "/tmp/country.mmdb", CountryField: new(CountryField("continent"))},
			},
			errs: []error{ErrGeoIPInvalidCountryField},
		},
		{
			name: "auto update missing everything",
			input: &GeoIPDatabases{
				ASN:        &GeoIPDatabase{Path: "/tmp/asn.mmdb"},
				AutoUpdate: &GeoIPAutoUpdate{},
			},
			errs: []error{ErrGeoIPNoAccountID, ErrGeoIPNoLicenseKeyEnv},
		},
		{
			name: "auto update license env not set",
			input: &GeoIPDatabases{
				ASN: &GeoIPDatabase{Path: "/tmp/asn.mmdb"},
				AutoUpdate: &GeoIPAutoUpdate{
					AccountID:     1234,
					LicenseKeyEnv: testLicenseKeyEnv,
				},
			},
			errs: []error{ErrGeoIPLicenseKeyEnvNotSet},
		},
		{
			name: "auto update bad interval and endpoint",
			input: &GeoIPDatabases{
				ASN: &GeoIPDatabase{Path: "/tmp/asn.mmdb"},
				AutoUpdate: &GeoIPAutoUpdate{
					AccountID:     1234,
					LicenseKeyEnv: testLicenseKeyEnv,
					Interval:      new("soon"),
					Endpoint:      new("updates.maxmind.com"),
				},
			},
			setEnv: true,
			errs:   []error{ErrGeoIPInvalidInterval, ErrGeoIPInvalidEndpoint},
		},
		{
			name: "auto update interval too short",
			input: &GeoIPDatabases{
				ASN: &GeoIPDatabase{Path: "/tmp/asn.mmdb"},
				AutoUpdate: &GeoIPAutoUpdate{
					AccountID:     1234,
					LicenseKeyEnv: testLicenseKeyEnv,
					Interval:      new("5m"),
				},
			},
			setEnv: true,
			errs:   []error{ErrGeoIPIntervalTooShort},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(testLicenseKeyEnv, "hunter2")
			} else {
				t.Setenv(testLicenseKeyEnv, "")
			}

			err := tt.input.Valid()

			if len(tt.errs) == 0 && err != nil {
				t.Fatalf("wanted no error, got: %v", err)
			}

			if len(tt.errs) != 0 && err == nil {
				t.Fatalf("wanted errors %v, got nil", tt.errs)
			}

			for _, want := range tt.errs {
				if !errors.Is(err, want) {
					t.Logf("want: %v", want)
					t.Logf("got:  %v", err)
					t.Error("got wrong error")
				}
			}
		})
	}
}

func TestGeoIPDatabasesDefaults(t *testing.T) {
	g := &GeoIPDatabases{
		ASN:        &GeoIPDatabase{Path: "/tmp/asn.mmdb"},
		Country:    &GeoIPDatabase{Path: "/tmp/country.mmdb"},
		AutoUpdate: &GeoIPAutoUpdate{},
	}

	if got := g.ASNEditionID(); got != DefaultGeoIPASNEditionID {
		t.Errorf("ASNEditionID: want %q, got %q", DefaultGeoIPASNEditionID, got)
	}

	if got := g.CountryEditionID(); got != DefaultGeoIPCountryEditionID {
		t.Errorf("CountryEditionID: want %q, got %q", DefaultGeoIPCountryEditionID, got)
	}

	if got := g.Country.GetCountryField(); got != CountryFieldCountry {
		t.Errorf("GetCountryField: want %q, got %q", CountryFieldCountry, got)
	}

	if got := g.AutoUpdate.GetInterval(); got != DefaultGeoIPUpdateInterval {
		t.Errorf("GetInterval: want %s, got %s", DefaultGeoIPUpdateInterval, got)
	}

	g.AutoUpdate.Interval = new("2h")
	if got := g.AutoUpdate.GetInterval(); got != 2*time.Hour {
		t.Errorf("GetInterval: want %s, got %s", 2*time.Hour, got)
	}
}

func TestGeoIPDatabasesLoad(t *testing.T) {
	fin, err := os.Open(filepath.Join("testdata", "geoip_databases.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer fin.Close() //nolint:errcheck

	c, err := Load(fin, "geoip_databases.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if c.GeoIP == nil {
		t.Fatal("wanted geoip block to be loaded, got nil")
	}

	if got := c.GeoIP.ASN.Path; got != "/var/lib/anubis/GeoLite2-ASN.mmdb" {
		t.Errorf("asn path: got %q", got)
	}

	if got := c.GeoIP.Country.GetCountryField(); got != CountryFieldRegisteredCountry {
		t.Errorf("country field: want %q, got %q", CountryFieldRegisteredCountry, got)
	}

	if c.GeoIP.AutoUpdate != nil {
		t.Errorf("auto_update: want nil, got %+v", c.GeoIP.AutoUpdate)
	}
}

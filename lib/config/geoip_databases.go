package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"
)

var (
	ErrGeoIPNoDatabases         = errors.New("config.GeoIPDatabases: must define at least one of asn or country")
	ErrGeoIPNoPath              = errors.New("config.GeoIPDatabase: must define path")
	ErrGeoIPCountryFieldOnASN   = errors.New("config.GeoIPDatabase: country_field can only be set on the country database")
	ErrGeoIPInvalidCountryField = errors.New("config.GeoIPDatabase: invalid country_field")
	ErrGeoIPEmptyEditionID      = errors.New("config.GeoIPDatabase: edition_id must not be empty when set")
	ErrGeoIPNoAccountID         = errors.New("config.GeoIPAutoUpdate: must define account_id")
	ErrGeoIPNoLicenseKeyEnv     = errors.New("config.GeoIPAutoUpdate: must define license_key_env")
	ErrGeoIPLicenseKeyEnvNotSet = errors.New("config.GeoIPAutoUpdate: environment variable named by license_key_env is not set")
	ErrGeoIPInvalidInterval     = errors.New("config.GeoIPAutoUpdate: invalid interval")
	ErrGeoIPIntervalTooShort    = errors.New("config.GeoIPAutoUpdate: interval is too short")
	ErrGeoIPInvalidEndpoint     = errors.New("config.GeoIPAutoUpdate: invalid endpoint")
)

const (
	DefaultGeoIPASNEditionID     = "GeoLite2-ASN"
	DefaultGeoIPCountryEditionID = "GeoLite2-Country"
	DefaultGeoIPUpdateInterval   = 24 * time.Hour
	MinimumGeoIPUpdateInterval   = time.Hour
	DefaultGeoIPCountryField     = CountryFieldCountry
)

// CountryField selects which country record in a MaxMind country database is
// used for geoip rules.
type CountryField string

const (
	// CountryFieldCountry is the country where MaxMind believes the IP address is located.
	CountryFieldCountry CountryField = "country"
	// CountryFieldRegisteredCountry is the country the IP address is registered to.
	CountryFieldRegisteredCountry CountryField = "registered_country"
)

func (cf CountryField) Valid() error {
	switch cf {
	case CountryFieldCountry, CountryFieldRegisteredCountry:
		return nil
	default:
		return fmt.Errorf("%w: %q, must be one of %q or %q", ErrGeoIPInvalidCountryField, string(cf), CountryFieldCountry, CountryFieldRegisteredCountry)
	}
}

// GeoIPDatabases configures the MaxMind databases used for asns and geoip
// rules and ASN logging.
type GeoIPDatabases struct {
	ASN        *GeoIPDatabase   `json:"asn,omitempty" yaml:"asn,omitempty"`
	Country    *GeoIPDatabase   `json:"country,omitempty" yaml:"country,omitempty"`
	AutoUpdate *GeoIPAutoUpdate `json:"auto_update,omitempty" yaml:"auto_update,omitempty"`
}

func (g *GeoIPDatabases) Valid() error {
	var errs []error

	if g.ASN == nil && g.Country == nil {
		errs = append(errs, ErrGeoIPNoDatabases)
	}

	if g.ASN != nil {
		if err := g.ASN.Valid(); err != nil {
			errs = append(errs, fmt.Errorf("asn: %w", err))
		}

		if g.ASN.CountryField != nil {
			errs = append(errs, fmt.Errorf("asn: %w", ErrGeoIPCountryFieldOnASN))
		}
	}

	if g.Country != nil {
		if err := g.Country.Valid(); err != nil {
			errs = append(errs, fmt.Errorf("country: %w", err))
		}
	}

	if g.AutoUpdate != nil {
		if err := g.AutoUpdate.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("config.GeoIPDatabases: invalid geoip settings:\n%w", errors.Join(errs...))
	}

	return nil
}

// ASNEditionID returns the MaxMind edition ID for the ASN database.
func (g *GeoIPDatabases) ASNEditionID() string {
	return g.ASN.editionID(DefaultGeoIPASNEditionID)
}

// CountryEditionID returns the MaxMind edition ID for the country database.
func (g *GeoIPDatabases) CountryEditionID() string {
	return g.Country.editionID(DefaultGeoIPCountryEditionID)
}

// GeoIPDatabase is a single MaxMind database on disk.
type GeoIPDatabase struct {
	Path         string        `json:"path" yaml:"path"`
	EditionID    *string       `json:"edition_id,omitempty" yaml:"edition_id,omitempty"`
	CountryField *CountryField `json:"country_field,omitempty" yaml:"country_field,omitempty"`
}

func (g *GeoIPDatabase) Valid() error {
	var errs []error

	if g.Path == "" {
		errs = append(errs, ErrGeoIPNoPath)
	}

	if g.EditionID != nil && *g.EditionID == "" {
		errs = append(errs, ErrGeoIPEmptyEditionID)
	}

	if g.CountryField != nil {
		if err := g.CountryField.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

// GetCountryField returns the configured country field or the default.
func (g *GeoIPDatabase) GetCountryField() CountryField {
	if g == nil || g.CountryField == nil {
		return DefaultGeoIPCountryField
	}

	return *g.CountryField
}

func (g *GeoIPDatabase) editionID(def string) string {
	if g == nil || g.EditionID == nil {
		return def
	}

	return *g.EditionID
}

// GeoIPAutoUpdate configures automatic database updates from MaxMind.
type GeoIPAutoUpdate struct {
	AccountID     int     `json:"account_id" yaml:"account_id"`
	LicenseKeyEnv string  `json:"license_key_env" yaml:"license_key_env"`
	Interval      *string `json:"interval,omitempty" yaml:"interval,omitempty"`
	Endpoint      *string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
}

func (g *GeoIPAutoUpdate) Valid() error {
	var errs []error

	if g.AccountID <= 0 {
		errs = append(errs, ErrGeoIPNoAccountID)
	}

	switch {
	case g.LicenseKeyEnv == "":
		errs = append(errs, ErrGeoIPNoLicenseKeyEnv)
	case os.Getenv(g.LicenseKeyEnv) == "":
		errs = append(errs, fmt.Errorf("%w: %s", ErrGeoIPLicenseKeyEnvNotSet, g.LicenseKeyEnv))
	}

	if g.Interval != nil {
		d, err := time.ParseDuration(*g.Interval)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("%w: %w", ErrGeoIPInvalidInterval, err))
		case d < MinimumGeoIPUpdateInterval:
			errs = append(errs, fmt.Errorf("%w: %s is less than %s", ErrGeoIPIntervalTooShort, d, MinimumGeoIPUpdateInterval))
		}
	}

	if g.Endpoint != nil {
		u, err := url.Parse(*g.Endpoint)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("%w: %w", ErrGeoIPInvalidEndpoint, err))
		case u.Scheme != "http" && u.Scheme != "https", u.Host == "":
			errs = append(errs, fmt.Errorf("%w: %q must be an absolute http or https URL", ErrGeoIPInvalidEndpoint, *g.Endpoint))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("auto_update: %w", errors.Join(errs...))
	}

	return nil
}

// GetInterval returns the parsed update interval or the default. Call Valid first.
func (g *GeoIPAutoUpdate) GetInterval() time.Duration {
	if g.Interval == nil {
		return DefaultGeoIPUpdateInterval
	}

	// XXX(Xe): already validated in Valid()
	d, _ := time.ParseDuration(*g.Interval)
	return d
}

// LicenseKey reads the license key from the configured environment variable.
func (g *GeoIPAutoUpdate) LicenseKey() string {
	return os.Getenv(g.LicenseKeyEnv)
}

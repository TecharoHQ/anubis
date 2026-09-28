// Package geoip looks up the autonomous system and country of IP addresses
// with MaxMind databases. It powers the asns and geoip bot rules and ASN
// logging.
package geoip

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/oschwald/maxminddb-golang/v2"
)

var (
	ErrWrongDatabaseType = errors.New("geoip: database is the wrong type")
	ErrCantOpenDatabase  = errors.New("geoip: can't open database")
)

// Kind is the kind of data a database holds.
type Kind string

const (
	KindASN     Kind = "ASN"
	KindCountry Kind = "Country"
)

// DB holds the ASN and country databases. Either may be absent. A nil *DB is
// valid and reports that nothing is loaded.
type DB struct {
	asn          *source
	country      *source
	countryField config.CountryField
}

// source is one database file. The reader is swapped atomically when the file
// is updated, so lookups never lock.
type source struct {
	kind    Kind
	path    string
	edition string
	reader  atomic.Pointer[maxminddb.Reader]

	// md5 and modTime are only touched during New and by the update goroutine.
	md5     string
	modTime time.Time
}

type asnRecord struct {
	Number       uint32 `maxminddb:"autonomous_system_number"`
	Organization string `maxminddb:"autonomous_system_organization"`
}

type countryRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	RegisteredCountry struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"registered_country"`
}

// New opens the databases in cfg. When auto_update is set, missing databases
// are downloaded before New returns. A background goroutine keeps the
// databases fresh until ctx is canceled.
func New(ctx context.Context, lg *slog.Logger, cfg *config.GeoIPDatabases) (*DB, error) {
	lg = lg.With("at", "geoip")

	db := &DB{
		countryField: cfg.Country.GetCountryField(),
	}

	var up *updater
	if cfg.AutoUpdate != nil {
		var err error
		up, err = newUpdater(cfg.AutoUpdate)
		if err != nil {
			return nil, err
		}
	}

	var errs []error

	if cfg.ASN != nil {
		db.asn = &source{kind: KindASN, path: cfg.ASN.Path, edition: cfg.ASNEditionID()}
	}

	if cfg.Country != nil {
		db.country = &source{kind: KindCountry, path: cfg.Country.Path, edition: cfg.CountryEditionID()}
	}

	for _, src := range db.sources() {
		err := src.load()
		if errors.Is(err, fs.ErrNotExist) && up != nil {
			lg.InfoContext(ctx, "database not found on disk, downloading it", "edition", src.edition, "path", src.path)
			_, err = up.update(ctx, src)
		}

		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	go db.run(ctx, lg, up)

	return db, nil
}

// FromReaders builds a DB from already opened readers. It does not watch
// or update anything. It is meant for tests and embedders.
func FromReaders(asn, country *maxminddb.Reader, countryField config.CountryField) *DB {
	db := &DB{countryField: countryField}

	if asn != nil {
		db.asn = &source{kind: KindASN}
		db.asn.reader.Store(asn)
	}

	if country != nil {
		db.country = &source{kind: KindCountry}
		db.country.reader.Store(country)
	}

	return db
}

func (db *DB) sources() []*source {
	var result []*source
	for _, src := range []*source{db.asn, db.country} {
		if src != nil {
			result = append(result, src)
		}
	}
	return result
}

// HasASN reports whether an ASN database is configured.
func (db *DB) HasASN() bool {
	return db != nil && db.asn != nil
}

// HasCountry reports whether a country database is configured.
func (db *DB) HasCountry() bool {
	return db != nil && db.country != nil
}

// LookupASN returns the autonomous system for addr. ok is false when the
// address is not publicly announced (such as private or reserved ranges) or
// no ASN database is loaded.
func (db *DB) LookupASN(addr netip.Addr) (asn uint32, org string, ok bool) {
	if !db.HasASN() {
		return 0, "", false
	}

	rdr := db.asn.reader.Load()
	if rdr == nil {
		return 0, "", false
	}

	var rec asnRecord
	res := rdr.Lookup(addr.Unmap())
	if !res.Found() {
		return 0, "", false
	}

	if err := res.Decode(&rec); err != nil || rec.Number == 0 {
		return 0, "", false
	}

	return rec.Number, rec.Organization, true
}

// LookupCountry returns the lowercase ISO 3166-1 alpha-2 country code for
// addr. ok is false when the address has no country or no country database is
// loaded.
func (db *DB) LookupCountry(addr netip.Addr) (cc string, ok bool) {
	if !db.HasCountry() {
		return "", false
	}

	rdr := db.country.reader.Load()
	if rdr == nil {
		return "", false
	}

	var rec countryRecord
	res := rdr.Lookup(addr.Unmap())
	if !res.Found() {
		return "", false
	}

	if err := res.Decode(&rec); err != nil {
		return "", false
	}

	primary, fallback := rec.Country.ISOCode, rec.RegisteredCountry.ISOCode
	if db.countryField == config.CountryFieldRegisteredCountry {
		primary, fallback = fallback, primary
	}

	switch {
	case primary != "":
		return strings.ToLower(primary), true
	case fallback != "":
		return strings.ToLower(fallback), true
	default:
		return "", false
	}
}

// load reads the database from disk and swaps it in.
func (src *source) load() error {
	fi, err := os.Stat(src.path)
	if err != nil {
		return fmt.Errorf("%w %s: %w", ErrCantOpenDatabase, src.path, err)
	}

	data, err := os.ReadFile(src.path)
	if err != nil {
		return fmt.Errorf("%w %s: %w", ErrCantOpenDatabase, src.path, err)
	}

	sum := md5.Sum(data)
	if err := src.swap(data, hex.EncodeToString(sum[:])); err != nil {
		return fmt.Errorf("%w %s: %w", ErrCantOpenDatabase, src.path, err)
	}

	src.modTime = fi.ModTime()
	return nil
}

// swap parses data as a database, checks that it is the right kind, and
// makes it the active reader.
//
// The database is held in memory instead of being mmapped so that an old
// reader stays valid for any lookup still using it after a swap. The garbage
// collector frees it once nothing refers to it.
func (src *source) swap(data []byte, sum string) error {
	rdr, err := maxminddb.OpenBytes(data)
	if err != nil {
		return err
	}

	if !strings.Contains(rdr.Metadata.DatabaseType, string(src.kind)) {
		return fmt.Errorf("%w: wanted a %s database, got %q", ErrWrongDatabaseType, src.kind, rdr.Metadata.DatabaseType)
	}

	src.reader.Store(rdr)
	src.md5 = sum
	databaseBuildTime.WithLabelValues(string(src.kind)).Set(float64(rdr.Metadata.BuildEpoch))

	return nil
}

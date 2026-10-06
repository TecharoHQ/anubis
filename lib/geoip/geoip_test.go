package geoip_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/geoip"
	"github.com/TecharoHQ/anubis/lib/geoip/geoiptest"
	"github.com/maxmind/geoipupdate/v8/client"
	"github.com/oschwald/maxminddb-golang/v2"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestLookup(t *testing.T) {
	db := geoiptest.DB(t)

	for _, tt := range []struct {
		name    string
		ip      string
		asn     uint32
		org     string
		asnOK   bool
		country string
		ccOK    bool
	}{
		{name: "cloudflare v4", ip: "1.1.1.1", asn: 13335, org: "Cloudflare, Inc.", asnOK: true, country: "us", ccOK: true},
		{name: "cloudflare v6", ip: "2606:4700::1111", asn: 13335, org: "Cloudflare, Inc.", asnOK: true, country: "us", ccOK: true},
		{name: "ipv4 mapped ipv6", ip: "::ffff:1.1.1.1", asn: 13335, org: "Cloudflare, Inc.", asnOK: true, country: "us", ccOK: true},
		{name: "registered country fallback", ip: "1.0.0.1", asn: 13335, org: "Cloudflare, Inc.", asnOK: true, country: "au", ccOK: true},
		{name: "canada", ip: "2.2.2.2", asn: 420, org: "test canada", asnOK: true, country: "ca", ccOK: true},
		{name: "not in database", ip: "8.8.8.8"},
		{name: "private", ip: "10.0.0.1"},
		{name: "loopback", ip: "127.0.0.1"},
		{name: "ipv6 loopback", ip: "::1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.ip)

			asn, org, ok := db.LookupASN(addr)
			if ok != tt.asnOK || asn != tt.asn || org != tt.org {
				t.Errorf("LookupASN: want (%d, %q, %v), got (%d, %q, %v)", tt.asn, tt.org, tt.asnOK, asn, org, ok)
			}

			cc, ok := db.LookupCountry(addr)
			if ok != tt.ccOK || cc != tt.country {
				t.Errorf("LookupCountry: want (%q, %v), got (%q, %v)", tt.country, tt.ccOK, cc, ok)
			}
		})
	}
}

func TestLookupCountryField(t *testing.T) {
	country, err := maxminddb.OpenBytes(geoiptest.CountryDatabase(t, geoiptest.DefaultRecords))
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name  string
		field config.CountryField
		ip    string
		want  string
	}{
		{name: "country", field: config.CountryFieldCountry, ip: "57.141.0.1", want: "us"},
		{name: "registered country", field: config.CountryFieldRegisteredCountry, ip: "57.141.0.1", want: "ie"},
		{name: "registered country falls back to country", field: config.CountryFieldRegisteredCountry, ip: "1.1.1.1", want: "us"},
		{name: "country falls back to registered country", field: config.CountryFieldCountry, ip: "1.0.0.1", want: "au"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := geoip.FromReaders(nil, country, tt.field)
			got, ok := db.LookupCountry(netip.MustParseAddr(tt.ip))
			if !ok || got != tt.want {
				t.Errorf("want %q, got %q (ok: %v)", tt.want, got, ok)
			}
		})
	}
}

func TestNilDB(t *testing.T) {
	var db *geoip.DB

	if db.HasASN() || db.HasCountry() {
		t.Error("nil DB should have no databases")
	}

	if _, _, ok := db.LookupASN(netip.MustParseAddr("1.1.1.1")); ok {
		t.Error("nil DB LookupASN should not find anything")
	}

	if _, ok := db.LookupCountry(netip.MustParseAddr("1.1.1.1")); ok {
		t.Error("nil DB LookupCountry should not find anything")
	}
}

func TestCheckers(t *testing.T) {
	db := geoiptest.DB(t)
	asnc := db.ASNCheckerFor([]uint32{13335})
	gipc := db.GeoIPCheckerFor([]string{"CA"})

	for _, tt := range []struct {
		name      string
		ip        string
		wantASN   bool
		wantGeoIP bool
	}{
		{name: "cloudflare", ip: "1.1.1.1", wantASN: true},
		{name: "canada", ip: "2.2.2.2", wantGeoIP: true},
		{name: "not in database", ip: "8.8.8.8"},
		{name: "private", ip: "10.0.0.1"},
		{name: "not an IP", ip: "taco"},
		{name: "empty", ip: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("X-Real-IP", tt.ip)

			got, err := asnc.Check(r)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.wantASN {
				t.Errorf("ASNChecker: want %v, got %v", tt.wantASN, got)
			}

			got, err = gipc.Check(r)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.wantGeoIP {
				t.Errorf("GeoIPChecker: want %v, got %v", tt.wantGeoIP, got)
			}
		})
	}

	if asnc.Hash() == db.ASNCheckerFor([]uint32{420}).Hash() {
		t.Error("ASN checkers for different ASNs have the same hash")
	}

	if gipc.Hash() == db.GeoIPCheckerFor([]string{"US"}).Hash() {
		t.Error("GeoIP checkers for different countries have the same hash")
	}
}

func TestNew(t *testing.T) {
	asnPath, countryPath := geoiptest.WriteFixtures(t)

	for _, tt := range []struct {
		name string
		cfg  *config.GeoIPDatabases
		err  error
	}{
		{
			name: "both",
			cfg: &config.GeoIPDatabases{
				ASN:     &config.GeoIPDatabase{Path: asnPath},
				Country: &config.GeoIPDatabase{Path: countryPath},
			},
		},
		{
			name: "asn only",
			cfg: &config.GeoIPDatabases{
				ASN: &config.GeoIPDatabase{Path: asnPath},
			},
		},
		{
			name: "missing file",
			cfg: &config.GeoIPDatabases{
				ASN: &config.GeoIPDatabase{Path: filepath.Join(t.TempDir(), "nope.mmdb")},
			},
			err: os.ErrNotExist,
		},
		{
			name: "swapped paths",
			cfg: &config.GeoIPDatabases{
				ASN:     &config.GeoIPDatabase{Path: countryPath},
				Country: &config.GeoIPDatabase{Path: asnPath},
			},
			err: geoip.ErrWrongDatabaseType,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, err := geoip.New(t.Context(), discardLogger(), tt.cfg)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Logf("want: %v", tt.err)
					t.Logf("got:  %v", err)
					t.Fatal("got wrong error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if _, _, ok := db.LookupASN(netip.MustParseAddr("1.1.1.1")); !ok {
				t.Error("wanted ASN lookup to succeed")
			}

			if got := db.HasCountry(); got != (tt.cfg.Country != nil) {
				t.Errorf("HasCountry: want %v, got %v", tt.cfg.Country != nil, got)
			}
		})
	}
}

var updatedRecords = append([]geoiptest.Record{
	{CIDR: "3.3.3.0/24", ASN: 999, Organization: "new network"},
}, geoiptest.DefaultRecords...)

// replaceFile swaps in new contents at path with a rename, the way
// geoipupdate does. Writing a mapped database in place would change the
// memory of the reader that is already loaded.
func replaceFile(t *testing.T, path string, data []byte) {
	t.Helper()

	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

func TestReloadFromDisk(t *testing.T) {
	asnPath, _ := geoiptest.WriteFixtures(t)

	db, err := geoip.New(t.Context(), discardLogger(), &config.GeoIPDatabases{
		ASN: &config.GeoIPDatabase{Path: asnPath},
	})
	if err != nil {
		t.Fatal(err)
	}

	addr := netip.MustParseAddr("3.3.3.3")
	if _, _, ok := db.LookupASN(addr); ok {
		t.Fatal("3.3.3.3 should not be in the first database")
	}

	// Unchanged file: nothing happens.
	db.CheckASNFile(t.Context(), discardLogger())

	replaceFile(t, asnPath, geoiptest.ASNDatabase(t, updatedRecords))
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(asnPath, future, future); err != nil {
		t.Fatal(err)
	}

	db.CheckASNFile(t.Context(), discardLogger())

	if asn, _, ok := db.LookupASN(addr); !ok || asn != 999 {
		t.Fatalf("wanted reloaded database to find AS999, got %d (ok: %v)", asn, ok)
	}

	// A corrupt file must not replace the loaded database.
	replaceFile(t, asnPath, []byte("not a database"))
	future = future.Add(time.Hour)
	if err := os.Chtimes(asnPath, future, future); err != nil {
		t.Fatal(err)
	}

	db.CheckASNFile(t.Context(), discardLogger())

	if asn, _, ok := db.LookupASN(addr); !ok || asn != 999 {
		t.Fatalf("corrupt file replaced the loaded database, got %d (ok: %v)", asn, ok)
	}
}

// fakeMaxMind serves the parts of the MaxMind update API that geoipupdate's
// client uses.
type fakeMaxMind struct {
	db       []byte
	md5      string
	status   int
	requests int
}

func newFakeMaxMind(t *testing.T, db []byte) (*fakeMaxMind, *httptest.Server) {
	t.Helper()

	sum := md5.Sum(db)
	f := &fakeMaxMind{db: db, md5: hex.EncodeToString(sum[:]), status: http.StatusOK}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /geoip/updates/metadata", func(w http.ResponseWriter, r *http.Request) {
		f.requests++
		user, pass, ok := r.BasicAuth()
		if !ok || user != "1234" || pass != "hunter2" || f.status != http.StatusOK {
			status := f.status
			if status == http.StatusOK {
				status = http.StatusUnauthorized
			}
			http.Error(w, `{"code":"INVALID_LICENSE_KEY"}`, status)
			return
		}

		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"databases": []map[string]string{{
				"date":       "2026-09-27",
				"edition_id": r.URL.Query().Get("edition_id"),
				"md5":        f.md5,
			}},
		})
	})
	mux.HandleFunc("GET /geoip/databases/{edition}/download", func(w http.ResponseWriter, r *http.Request) {
		edition := r.PathValue("edition")

		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)
		name := edition + "_20260927/" + edition + ".mmdb"
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(f.db))}) //nolint:errcheck
		tw.Write(f.db)                                                               //nolint:errcheck
		tw.Close()                                                                   //nolint:errcheck
		gw.Close()                                                                   //nolint:errcheck

		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Write(buf.Bytes()) //nolint:errcheck
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return f, srv
}

func autoUpdateConfig(t *testing.T, endpoint string) *config.GeoIPAutoUpdate {
	t.Helper()
	t.Setenv("ANUBIS_TEST_MAXMIND_LICENSE_KEY", "hunter2")

	return &config.GeoIPAutoUpdate{
		AccountID:     1234,
		LicenseKeyEnv: "ANUBIS_TEST_MAXMIND_LICENSE_KEY",
		Endpoint:      &endpoint,
	}
}

func TestNewDownloadsMissingDatabase(t *testing.T) {
	_, srv := newFakeMaxMind(t, geoiptest.ASNDatabase(t, geoiptest.DefaultRecords))
	asnPath := filepath.Join(t.TempDir(), "sub", "GeoLite2-ASN.mmdb")

	db, err := geoip.New(t.Context(), discardLogger(), &config.GeoIPDatabases{
		ASN:        &config.GeoIPDatabase{Path: asnPath},
		AutoUpdate: autoUpdateConfig(t, srv.URL),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, ok := db.LookupASN(netip.MustParseAddr("1.1.1.1")); !ok {
		t.Error("wanted downloaded database to be loaded")
	}

	if _, err := os.Stat(asnPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the configured path should not be written, stat: %v", err)
	}

	versions := versionFiles(t, asnPath)
	if len(versions) != 1 {
		t.Fatalf("wanted one timestamped download, got %v", versions)
	}
}

// versionFiles lists the timestamped downloads and staging files next to path.
func versionFiles(t *testing.T, path string) []string {
	t.Helper()

	matches, err := filepath.Glob(path + ".*")
	if err != nil {
		t.Fatal(err)
	}

	return matches
}

func TestUpdate(t *testing.T) {
	for _, tt := range []struct {
		name        string
		serve       func(t *testing.T) []byte
		badMD5      bool
		status      int
		wantUpdated bool
		wantAS999   bool
		wantErr     bool
		wantHTTP    int
	}{
		{
			name:  "no update available",
			serve: func(t *testing.T) []byte { return geoiptest.ASNDatabase(t, geoiptest.DefaultRecords) },
		},
		{
			name:        "update available",
			serve:       func(t *testing.T) []byte { return geoiptest.ASNDatabase(t, updatedRecords) },
			wantUpdated: true,
			wantAS999:   true,
		},
		{
			name:    "corrupt download",
			serve:   func(t *testing.T) []byte { return []byte("not a database") },
			wantErr: true,
		},
		{
			name:    "wrong database type",
			serve:   func(t *testing.T) []byte { return geoiptest.CountryDatabase(t, updatedRecords) },
			wantErr: true,
		},
		{
			name:    "checksum mismatch",
			serve:   func(t *testing.T) []byte { return geoiptest.ASNDatabase(t, updatedRecords) },
			badMD5:  true,
			wantErr: true,
		},
		{
			name:     "bad license key",
			serve:    func(t *testing.T) []byte { return geoiptest.ASNDatabase(t, updatedRecords) },
			status:   http.StatusUnauthorized,
			wantErr:  true,
			wantHTTP: http.StatusUnauthorized,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			asnPath, _ := geoiptest.WriteFixtures(t)
			original, err := os.ReadFile(asnPath)
			if err != nil {
				t.Fatal(err)
			}

			fake, srv := newFakeMaxMind(t, tt.serve(t))
			if tt.badMD5 {
				fake.md5 = "00000000000000000000000000000000"
			}
			if tt.status != 0 {
				fake.status = tt.status
			}

			cfg := autoUpdateConfig(t, srv.URL)
			db, err := geoip.New(t.Context(), discardLogger(), &config.GeoIPDatabases{
				ASN:        &config.GeoIPDatabase{Path: asnPath},
				AutoUpdate: cfg,
			})
			if err != nil {
				t.Fatal(err)
			}

			updated, err := db.UpdateASN(t.Context(), cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("wantErr: %v, got: %v", tt.wantErr, err)
			}

			if tt.wantHTTP != 0 {
				var httpErr client.HTTPError
				if !errors.As(err, &httpErr) || httpErr.StatusCode != tt.wantHTTP {
					t.Errorf("wanted HTTP %d error, got: %v", tt.wantHTTP, err)
				}
			}

			if updated != tt.wantUpdated {
				t.Errorf("updated: want %v, got %v", tt.wantUpdated, updated)
			}

			_, _, found := db.LookupASN(netip.MustParseAddr("3.3.3.3"))
			if found != tt.wantAS999 {
				t.Errorf("AS999 lookup: want %v, got %v", tt.wantAS999, found)
			}

			if _, _, ok := db.LookupASN(netip.MustParseAddr("1.1.1.1")); !ok {
				t.Error("existing data should still be served")
			}

			onDisk, err := os.ReadFile(asnPath)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(onDisk, original) {
				t.Error("the configured path was modified, updates must go to new files")
			}

			versions := versionFiles(t, asnPath)
			wantVersions := 0
			if tt.wantUpdated {
				wantVersions = 1
			}
			if len(versions) != wantVersions {
				t.Errorf("timestamped files: want %d, got %v", wantVersions, versions)
			}
		})
	}
}

func TestContext(t *testing.T) {
	if _, ok := geoip.FromContext(t.Context()); ok {
		t.Error("empty context should not have a DB")
	}

	db := geoiptest.DB(t)
	got, ok := geoip.FromContext(geoip.With(t.Context(), db))
	if !ok || got != db {
		t.Error("wanted DB back from context")
	}

	if _, ok := geoip.FromContext(geoip.With(t.Context(), nil)); ok {
		t.Error("nil DB in context should not count")
	}
}

func TestVersionedStartupAndCleanup(t *testing.T) {
	asnPath, _ := geoiptest.WriteFixtures(t)

	older := asnPath + ".1700000000"
	newer := asnPath + ".1800000000"
	for fname, records := range map[string][]geoiptest.Record{
		older: geoiptest.DefaultRecords,
		newer: updatedRecords,
	} {
		if err := os.WriteFile(fname, geoiptest.ASNDatabase(t, records), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Files that must be ignored when picking the newest version.
	for _, fname := range []string{asnPath + ".1900000000.tmp", asnPath + ".bak", asnPath + ".1900000000x"} {
		if err := os.WriteFile(fname, []byte("junk"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	_, srv := newFakeMaxMind(t, geoiptest.ASNDatabase(t, updatedRecords))
	cfg := autoUpdateConfig(t, srv.URL)

	db, err := geoip.New(t.Context(), discardLogger(), &config.GeoIPDatabases{
		ASN:        &config.GeoIPDatabase{Path: asnPath},
		AutoUpdate: cfg,
	})
	if err != nil {
		t.Fatal(err)
	}

	if asn, _, ok := db.LookupASN(netip.MustParseAddr("3.3.3.3")); !ok || asn != 999 {
		t.Fatalf("wanted newest version to be loaded, got AS%d (ok: %v)", asn, ok)
	}

	for fname, want := range map[string]bool{
		asnPath:                     true,
		newer:                       true,
		older:                       false,
		asnPath + ".1900000000.tmp": false,
		asnPath + ".bak":            true,
		asnPath + ".1900000000x":    true,
	} {
		_, err := os.Stat(fname)
		if got := err == nil; got != want {
			t.Errorf("%s exists: want %v, got %v", filepath.Base(fname), want, got)
		}
	}

	// The server has the same database, so no new file appears.
	updated, err := db.UpdateASN(t.Context(), cfg)
	if err != nil || updated {
		t.Fatalf("wanted no update, got updated=%v err=%v", updated, err)
	}
}

func TestVersionAfterClockStep(t *testing.T) {
	asnPath, _ := geoiptest.WriteFixtures(t)

	// A version from the future, as if the clock stepped backwards since.
	future := asnPath + ".9999999999"
	if err := os.WriteFile(future, geoiptest.ASNDatabase(t, geoiptest.DefaultRecords), 0o644); err != nil {
		t.Fatal(err)
	}

	_, srv := newFakeMaxMind(t, geoiptest.ASNDatabase(t, updatedRecords))
	cfg := autoUpdateConfig(t, srv.URL)

	db, err := geoip.New(t.Context(), discardLogger(), &config.GeoIPDatabases{
		ASN:        &config.GeoIPDatabase{Path: asnPath},
		AutoUpdate: cfg,
	})
	if err != nil {
		t.Fatal(err)
	}

	if updated, err := db.UpdateASN(t.Context(), cfg); err != nil || !updated {
		t.Fatalf("wanted an update, got updated=%v err=%v", updated, err)
	}

	if _, err := os.Stat(asnPath + ".10000000000"); err != nil {
		t.Errorf("wanted the new version to sort after the future one: %v", err)
	}
}

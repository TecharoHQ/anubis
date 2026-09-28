package geoip

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/maxmind/geoipupdate/v8/client"
)

var (
	ErrChecksumMismatch = errors.New("geoip: downloaded database checksum does not match")
	ErrDatabaseTooLarge = errors.New("geoip: downloaded database is too large")
)

const (
	// maxDatabaseSize caps how much of a download is written to disk.
	// GeoIP2-City, the largest common edition, is well under this.
	maxDatabaseSize = 512 << 20

	// fileCheckInterval is how often database files are checked for changes
	// when automatic updates are off.
	fileCheckInterval = 5 * time.Minute
)

type updater struct {
	cli      client.Client
	interval time.Duration
}

func newUpdater(cfg *config.GeoIPAutoUpdate) (*updater, error) {
	opts := []client.Option{
		client.WithHTTPClient(&http.Client{
			Timeout:   5 * time.Minute,
			Transport: userAgentTransport{next: http.DefaultTransport},
		}),
	}

	if cfg.Endpoint != nil {
		opts = append(opts, client.WithEndpoint(*cfg.Endpoint))
	}

	cli, err := client.New(cfg.AccountID, cfg.LicenseKey(), opts...)
	if err != nil {
		return nil, fmt.Errorf("geoip: can't create MaxMind update client: %w", err)
	}

	return &updater{
		cli:      cli,
		interval: cfg.GetInterval(),
	}, nil
}

// update checks MaxMind for a newer copy of src. When there is one, it
// writes the download next to src.path, verifies it, renames it over
// src.path, and swaps it in. It reports whether a new database was installed.
func (u *updater) update(ctx context.Context, src *source) (bool, error) {
	resp, err := u.cli.Download(ctx, src.edition, src.md5)
	if err != nil {
		return false, fmt.Errorf("geoip: can't download %s: %w", src.edition, err)
	}
	defer resp.Reader.Close() //nolint:errcheck

	if !resp.UpdateAvailable {
		return false, nil
	}

	tmpName, gotMD5, err := writeTemp(src.path, resp.Reader)
	if err != nil {
		return false, fmt.Errorf("geoip: can't save %s download: %w", src.edition, err)
	}
	defer os.Remove(tmpName) //nolint:errcheck // fails harmlessly once renamed

	if gotMD5 != resp.MD5 {
		return false, fmt.Errorf("%w: %s: want %s, got %s", ErrChecksumMismatch, src.edition, resp.MD5, gotMD5)
	}

	// Map and check the new database before it replaces the old file, so a
	// bad download never replaces a good one. The mapping stays valid across
	// the rename because it refers to the file, not the name.
	rdr, err := src.open(tmpName)
	if err != nil {
		return false, fmt.Errorf("geoip: downloaded %s is not valid: %w", src.edition, err)
	}

	if err := os.Rename(tmpName, src.path); err != nil {
		rdr.Close() //nolint:errcheck
		return false, fmt.Errorf("geoip: can't replace %s: %w", src.path, err)
	}

	src.install(rdr, gotMD5)

	if fi, err := os.Stat(src.path); err == nil {
		src.modTime = fi.ModTime()
	}

	return true, nil
}

// writeTemp streams r into a synced temporary file next to path and returns
// its name and MD5 checksum. The caller removes the file.
func writeTemp(path string, r io.Reader) (string, string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", "", err
	}

	h := md5.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, maxDatabaseSize+1))
	if err == nil && n > maxDatabaseSize {
		err = fmt.Errorf("%w: larger than %d bytes", ErrDatabaseTooLarge, maxDatabaseSize)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644)
	}

	if err != nil {
		os.Remove(tmp.Name()) //nolint:errcheck
		return "", "", err
	}

	return tmp.Name(), hex.EncodeToString(h.Sum(nil)), nil
}

// run keeps the databases fresh until ctx is canceled. With an updater it
// polls MaxMind. Without one it reloads files that change on disk, such as
// when an external geoipupdate job replaces them.
func (db *DB) run(ctx context.Context, lg *slog.Logger, up *updater) {
	interval := fileCheckInterval
	if up != nil {
		interval = up.interval
	}

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		for _, src := range db.sources() {
			if up != nil {
				db.checkUpdate(ctx, lg, up, src)
			} else {
				db.checkFile(ctx, lg, src)
			}
		}
	}
}

func (db *DB) checkUpdate(ctx context.Context, lg *slog.Logger, up *updater, src *source) {
	lg = lg.With("edition", src.edition, "path", src.path)

	updated, err := up.update(ctx, src)
	if err != nil {
		updateErrors.WithLabelValues(string(src.kind)).Inc()

		var httpErr client.HTTPError
		if errors.As(err, &httpErr) && (httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden) {
			lg.ErrorContext(ctx, "MaxMind rejected the account ID or license key, please check the geoip auto_update settings", "err", err, "actionable", true)
			return
		}

		lg.WarnContext(ctx, "can't update geoip database, keeping the current one", "err", err)
		return
	}

	if updated {
		lg.InfoContext(ctx, "updated geoip database")
	}
}

func (db *DB) checkFile(ctx context.Context, lg *slog.Logger, src *source) {
	lg = lg.With("path", src.path)

	fi, err := os.Stat(src.path)
	if err != nil {
		updateErrors.WithLabelValues(string(src.kind)).Inc()
		lg.WarnContext(ctx, "can't stat geoip database, keeping the current one", "err", err)
		return
	}

	if fi.ModTime().Equal(src.modTime) {
		return
	}

	if err := src.load(); err != nil {
		updateErrors.WithLabelValues(string(src.kind)).Inc()
		lg.WarnContext(ctx, "can't reload geoip database, keeping the current one", "err", err)
		return
	}

	lg.InfoContext(ctx, "reloaded geoip database from disk")
}

type userAgentTransport struct {
	next http.RoundTripper
}

func (u userAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("User-Agent", fmt.Sprint("Techaro/anubis:", anubis.Version, " ", r.Header.Get("User-Agent")))
	return u.next.RoundTrip(r)
}

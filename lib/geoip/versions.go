package geoip

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Automatic updates never rename over or rewrite a database that is mapped.
// Each download is saved as its own file named after the configured path with
// the Unix timestamp of the update appended, such as
// GeoLite2-ASN.mmdb.1790496947. The newest one is loaded at startup, and older
// ones are removed once nothing should be using them.

const tmpSuffix = ".tmp"

// versionPath returns the file name for a download made at ts.
func (src *source) versionPath(ts int64) string {
	return src.path + "." + strconv.FormatInt(ts, 10)
}

// versions lists the downloaded copies of src, oldest first, and any staging
// files left behind by interrupted downloads.
func (src *source) versions() (versions, staging []string, err error) {
	dir := filepath.Dir(src.path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	type version struct {
		ts   int64
		path string
	}

	prefix := filepath.Base(src.path) + "."
	var found []version

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), tmpSuffix) {
			staging = append(staging, filepath.Join(dir, e.Name()))
			continue
		}

		if ts, ok := parseVersion(e.Name(), prefix); ok {
			found = append(found, version{ts: ts, path: filepath.Join(dir, e.Name())})
		}
	}

	slices.SortFunc(found, func(a, b version) int {
		return cmp.Compare(a.ts, b.ts)
	})

	for _, v := range found {
		versions = append(versions, v.path)
	}

	return versions, staging, nil
}

func parseVersion(name, prefix string) (int64, bool) {
	suffix, ok := strings.CutPrefix(name, prefix)
	if !ok || suffix == "" {
		return 0, false
	}

	for _, r := range suffix {
		if r < '0' || r > '9' {
			return 0, false
		}
	}

	ts, err := strconv.ParseInt(suffix, 10, 64)
	return ts, err == nil
}

// latest returns the newest downloaded copy of src, if any.
func (src *source) latest() (string, bool) {
	vs, _, err := src.versions()
	if err != nil || len(vs) == 0 {
		return "", false
	}

	return vs[len(vs)-1], true
}

// cleanup removes downloaded copies of src other than the one in use, and
// staging files left behind by interrupted downloads. Only the goroutine that
// owns updates may call it.
//
// On Windows a file cannot be removed while a reader still maps it. Those
// removals fail and are tried again after the next update check.
func (src *source) cleanup() error {
	vs, staging, err := src.versions()
	if err != nil {
		return err
	}

	var errs []error

	for _, f := range append(vs, staging...) {
		if f == src.current {
			continue
		}

		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("geoip: can't remove old %s databases: %w", src.edition, errors.Join(errs...))
	}

	return nil
}

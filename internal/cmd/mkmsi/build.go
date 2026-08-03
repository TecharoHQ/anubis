package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// ErrWixlDroppedAttribute reports that wixl did not understand part of the wxs.
//
// wixl writes a warning to stderr and then exits 0 with an installer that is
// missing whatever it did not understand, so its exit status cannot be trusted
// on its own.
var ErrWixlDroppedAttribute = errors.New("mkmsi: wixl rejected part of the wxs")

// zipNameRe pulls the version and architecture out of a yeet zip name such as
// anubis-1.26.2-1-gec3cce8a-dev-windows-amd64.zip.
var zipNameRe = regexp.MustCompile(`^anubis-(.+)-windows-(amd64|arm64)\.zip$`)

// versionFromZipName extracts the Anubis version from a zip file's name.
func versionFromZipName(name string) (string, error) {
	m := zipNameRe.FindStringSubmatch(filepath.Base(name))
	if m == nil {
		return "", fmt.Errorf("%w: %q is not an Anubis Windows zip", ErrBadVersion, name)
	}

	return m[1], nil
}

// archFromZipName extracts the Go architecture from a zip file's name.
func archFromZipName(name string) (string, error) {
	m := zipNameRe.FindStringSubmatch(filepath.Base(name))
	if m == nil {
		return "", fmt.Errorf("%w: %q is not an Anubis Windows zip", ErrBadVersion, name)
	}

	return m[2], nil
}

// checkWixlOutput fails when wixl reported that it could not handle something.
func checkWixlOutput(stderr string) error {
	for line := range strings.SplitSeq(stderr, "\n") {
		if strings.Contains(line, "CRITICAL") {
			return fmt.Errorf("%w: %s", ErrWixlDroppedAttribute, strings.TrimSpace(line))
		}
	}

	return nil
}

// unzip extracts src into dest, stripping the single top level directory that
// yeet puts in its archives. It returns the list of extracted file paths,
// relative to dest and sorted, so downstream steps are reproducible, along
// with the single modification time yeet stamped every entry with.
//
// That shared mtime is handed back rather than rediscovered so files this
// program generates itself after unzipping -- the config templates
// stageConfigTemplates writes -- can be stamped with it too. Without that,
// only files that came straight out of the zip would have a reproducible
// mtime, and the CAB archive wixl builds embeds every file's mtime
// regardless of where it came from.
func unzip(src, dest string) ([]string, time.Time, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("mkmsi: cannot open %s: %w", src, err)
	}
	defer func() { _ = r.Close() }()

	if len(r.File) == 0 {
		return nil, time.Time{}, fmt.Errorf("mkmsi: %s has no entries", src)
	}

	// yeet stamps every entry in the zip with the same mtime; verified
	// empirically across a real build's 151 entries. Any entry's mtime is
	// therefore as good as any other's.
	srcMTime := r.File[0].Modified

	var names []string

	for _, f := range r.File {
		// yeet wraps everything in one directory named after the package.
		rel := f.Name
		if i := strings.IndexByte(rel, '/'); i >= 0 {
			rel = rel[i+1:]
		}

		if rel == "" {
			continue
		}

		// Reject traversal before touching the filesystem: the zip is built
		// locally, but this program also runs over files from CI artifacts.
		out := filepath.Join(dest, filepath.FromSlash(rel))
		if !strings.HasPrefix(out, filepath.Clean(dest)+string(os.PathSeparator)) {
			return nil, time.Time{}, fmt.Errorf("mkmsi: refusing path %q outside %s", f.Name, dest)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(out, 0o755); err != nil {
				return nil, time.Time{}, fmt.Errorf("mkmsi: cannot create %s: %w", out, err)
			}
			continue
		}

		if err := extractOne(f, out); err != nil {
			return nil, time.Time{}, err
		}

		names = append(names, rel)
	}

	slices.Sort(names)

	return names, srcMTime, nil
}

// extractOne writes a single zip entry to disk.
func extractOne(f *zip.File, out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("mkmsi: cannot create %s: %w", filepath.Dir(out), err)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("mkmsi: cannot read %s from zip: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	w, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode().Perm()|0o200)
	if err != nil {
		return fmt.Errorf("mkmsi: cannot create %s: %w", out, err)
	}

	if _, err := io.Copy(w, rc); err != nil {
		_ = w.Close()
		return fmt.Errorf("mkmsi: cannot write %s: %w", out, err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("mkmsi: cannot close %s: %w", out, err)
	}

	// yeet pins every zip entry's modification time to a fixed epoch so its
	// zips are byte-identical across builds. os.OpenFile above stamps the
	// file with the current wall clock instead, and the CAB archive wixl
	// builds embeds each file's mtime -- so without restoring it here, the
	// CAB stream (and therefore the MSI) differs on every build even when
	// the input zip is identical. This must run after w.Close(), not
	// before: closing a file does not touch its mtime, but running before
	// the copy finishes would use the file's creation time instead.
	if err := os.Chtimes(out, f.Modified, f.Modified); err != nil {
		return fmt.Errorf("mkmsi: cannot set mtime on %s: %w", out, err)
	}

	return nil
}

// concatFiles writes the contents of srcs, in order, to dest, then stamps it
// with mtime.
//
// The stamp matters for the same reason extractOne restores one on every
// file it extracts from the zip: dest is a File the MSI installs, so its
// mtime is embedded in the CAB archive wixl builds. Left at the wall clock
// os.WriteFile would otherwise use, it would make the CAB -- and therefore
// the MSI -- differ on every build even when every other input is identical.
// mtime is srcMTime, the single mtime yeet stamped every entry in the source
// zip with (see unzip), so these generated files end up sharing it rather
// than inventing a separate fixed point in time of their own.
func concatFiles(dest string, mtime time.Time, srcs ...string) error {
	var buf []byte

	for _, src := range srcs {
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("mkmsi: cannot read %s: %w", src, err)
		}

		buf = append(buf, data...)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkmsi: cannot create %s: %w", filepath.Dir(dest), err)
	}

	if err := os.WriteFile(dest, buf, 0o644); err != nil {
		return fmt.Errorf("mkmsi: cannot write %s: %w", dest, err)
	}

	if err := os.Chtimes(dest, mtime, mtime); err != nil {
		return fmt.Errorf("mkmsi: cannot set mtime on %s: %w", dest, err)
	}

	return nil
}

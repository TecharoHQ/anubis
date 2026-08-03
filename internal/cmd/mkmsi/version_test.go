package main

import (
	"errors"
	"testing"
)

func TestMSIVersion(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{name: "plain release", in: "1.26.2", want: "1.26.2.0"},
		{name: "v prefix", in: "v1.26.2", want: "1.26.2.0"},
		{name: "dev build", in: "1.26.2-1-gec3cce8a-dev", want: "1.26.2.1"},
		{name: "dev build many commits", in: "1.26.2-137-gdeadbee", want: "1.26.2.137"},
		// Prereleases share a ProductVersion with their release; only the
		// ProductCode tells them apart. See versionFields.
		{name: "prerelease", in: "1.27.0-pre1", want: "1.27.0.0"},
		{name: "prerelease with v prefix", in: "v1.27.0-pre2", want: "1.27.0.0"},
		{name: "dev build off a prerelease", in: "1.27.0-pre1-4-gec3cce8a-dev", want: "1.27.0.4"},
		{name: "prerelease number is not a version component", in: "1.27.0-pre256", want: "1.27.0.0"},
		{name: "major too big", in: "256.0.0", wantErr: ErrVersionOutOfRange},
		{name: "minor too big", in: "1.256.0", wantErr: ErrVersionOutOfRange},
		{name: "patch too big", in: "1.0.65536", wantErr: ErrVersionOutOfRange},
		{name: "revision too big", in: "1.0.0-65536-gdeadbee", wantErr: ErrVersionOutOfRange},
		{name: "empty", in: "", wantErr: ErrBadVersion},
		{name: "not a version", in: "devel", wantErr: ErrBadVersion},
		{name: "two components", in: "1.26", wantErr: ErrBadVersion},
		{name: "four components", in: "1.26.2.4", wantErr: ErrBadVersion},
		{name: "trailing garbage", in: "1.26.2extra", wantErr: ErrBadVersion},
		{name: "garbage after commit hash", in: "1.26.2-1-gdead-devXXX", wantErr: ErrBadVersion},
		{name: "prerelease without a number", in: "1.27.0-pre", wantErr: ErrBadVersion},
		{name: "unknown prerelease flavour", in: "1.27.0-rc1", wantErr: ErrBadVersion},
		{name: "prerelease after commit count", in: "1.27.0-4-gdeadbee-pre1", wantErr: ErrBadVersion},
		{name: "overflows an int", in: "99999999999999999999.0.0", wantErr: ErrVersionOutOfRange},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := msiVersion(tt.in)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("msiVersion(%q): got error %v, want %v", tt.in, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("msiVersion(%q): unexpected error: %v", tt.in, err)
			}

			if got != tt.want {
				t.Errorf("msiVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestProductCodeIsDeterministic(t *testing.T) {
	const version = "1.26.2-1-gec3cce8a-dev"

	first, err := productCode(version, "amd64")
	if err != nil {
		t.Fatalf("productCode(%q): %v", version, err)
	}

	second, err := productCode(version, "amd64")
	if err != nil {
		t.Fatalf("productCode(%q): %v", version, err)
	}

	if first != second {
		t.Errorf("productCode is not deterministic: %q != %q", first, second)
	}
}

func TestProductCodeVariesByVersion(t *testing.T) {
	a, err := productCode("1.26.2", "amd64")
	if err != nil {
		t.Fatalf("productCode: %v", err)
	}

	b, err := productCode("1.26.3", "amd64")
	if err != nil {
		t.Fatalf("productCode: %v", err)
	}

	if a == b {
		t.Errorf("different versions produced the same ProductCode %q", a)
	}
}

func TestProductCodeVariesByPrerelease(t *testing.T) {
	pre, err := productCode("1.27.0-pre1", "amd64")
	if err != nil {
		t.Fatalf("productCode: %v", err)
	}

	release, err := productCode("1.27.0", "amd64")
	if err != nil {
		t.Fatalf("productCode: %v", err)
	}

	// A prerelease and its release compare equal as MSI ProductVersions, so
	// the ProductCode is the only thing that keeps them from looking like the
	// same installed package to Windows.
	if pre == release {
		t.Errorf("1.27.0-pre1 and 1.27.0 produced the same ProductCode %q", pre)
	}
}

func TestProductCodeVariesByArch(t *testing.T) {
	const version = "1.26.2"

	amd64, err := productCode(version, "amd64")
	if err != nil {
		t.Fatalf("productCode: %v", err)
	}

	arm64, err := productCode(version, "arm64")
	if err != nil {
		t.Fatalf("productCode: %v", err)
	}

	// Sharing a ProductCode across architectures makes Windows treat the two
	// packages as one product, so installing either one uninstalls the other.
	if amd64 == arm64 {
		t.Errorf("amd64 and arm64 produced the same ProductCode %q", amd64)
	}
}

func TestProductCodeRejectsBadVersion(t *testing.T) {
	if _, err := productCode("devel", "amd64"); !errors.Is(err, ErrBadVersion) {
		t.Errorf("productCode(\"devel\"): got %v, want %v", err, ErrBadVersion)
	}
}

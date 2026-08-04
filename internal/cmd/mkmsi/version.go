package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/google/uuid"
)

var (
	ErrBadVersion        = errors.New("mkmsi: cannot parse version")
	ErrVersionOutOfRange = errors.New("mkmsi: version component out of range")
)

// upgradeCode is the fixed MSI UpgradeCode for Anubis. Windows uses it to
// relate one installed version to another. It must never change: changing it
// makes every future installer fail to upgrade existing installs.
const upgradeCode = "92f5fc1f-f6f2-4bd1-bbef-79b8a46c665f"

// versionRe matches the versions yeet produces, such as "1.26.2" for a tagged
// release, "1.27.0-pre1" for a tagged prerelease, and
// "1.26.2-1-gec3cce8a-dev" for a build off a later commit.
//
// It is anchored at both ends on purpose. Without the trailing anchor a string
// like "1.26.2.4" matches its prefix and silently becomes 1.26.2.0, turning a
// version this code does not understand into a plausible wrong answer instead
// of a build failure.
var versionRe = regexp.MustCompile(`^v?(?P<major>\d+)\.(?P<minor>\d+)\.(?P<patch>\d+)(?:-pre(?P<pre>\d+))?(?:-(?P<commits>\d+)-g[0-9a-fA-F]+)?(?:-dev)?$`)

// versionFields maps the numeric fields of an MSI ProductVersion onto the
// capture groups of versionRe, in the order they appear in the output, along
// with the maximum value MSI allows in each.
//
// The prerelease number has no field of its own: MSI compares only
// major.minor.build when deciding whether one package upgrades another, and
// there is no way to express "1.27.0-pre1 sorts below 1.27.0" in three
// unsigned numbers. It therefore reaches the installer only through the
// ProductCode, which hashes the raw version string, so a prerelease and its
// release are distinct products that share a ProductVersion. The wxs sets
// AllowSameVersionUpgrades for exactly this reason, so installing 1.27.0 over
// 1.27.0-pre1 still removes the prerelease instead of leaving both registered.
var versionFields = []struct {
	name  string
	group string
	limit int
}{
	{name: "major", group: "major", limit: 255},
	{name: "minor", group: "minor", limit: 255},
	{name: "build", group: "patch", limit: 65535},
	{name: "revision", group: "commits", limit: 65535},
}

// msiVersion converts a yeet version string into an MSI ProductVersion.
//
// MSI requires a numeric major.minor.build.revision, where major and minor are
// at most 255 and build and revision are at most 65535. The commit count from a
// dev build becomes the revision; a tagged release gets revision 0.
//
// Note that Windows ignores the revision field when deciding whether one
// package upgrades another, so a dev build and its base release look identical
// to the installer.
func msiVersion(in string) (string, error) {
	m := versionRe.FindStringSubmatch(in)
	if m == nil {
		return "", fmt.Errorf("%w: %q", ErrBadVersion, in)
	}

	parts := make([]int, len(versionFields))

	for i, f := range versionFields {
		raw := m[versionRe.SubexpIndex(f.group)]
		if raw == "" {
			continue
		}

		n, err := strconv.Atoi(raw)
		if err != nil {
			// A digit string too long for an int is semantically out of
			// range, not malformed. Callers distinguish the two.
			if errors.Is(err, strconv.ErrRange) {
				return "", fmt.Errorf("%w: %s %q does not fit in an int", ErrVersionOutOfRange, f.name, raw)
			}
			return "", fmt.Errorf("%w: %s %q: %w", ErrBadVersion, f.name, raw, err)
		}

		if n > f.limit {
			return "", fmt.Errorf("%w: %s is %d, maximum is %d", ErrVersionOutOfRange, f.name, n, f.limit)
		}

		parts[i] = n
	}

	return fmt.Sprintf("%d.%d.%d.%d", parts[0], parts[1], parts[2], parts[3]), nil
}

// productCode derives the MSI ProductCode for a version and architecture.
// Every distinct version needs a distinct ProductCode, and rebuilding the
// same commit must produce the same one, so it is a UUIDv5 of the version and
// architecture in the UpgradeCode's namespace rather than a random UUID.
//
// The architecture is part of the hash input because it, like the version,
// must vary the ProductCode: two packages built for different architectures
// still install to the same UpgradeCode family, so if they shared a
// ProductCode too, Windows would treat them as the same product and
// installing one would silently uninstall the other.
func productCode(version, arch string) (string, error) {
	if _, err := msiVersion(version); err != nil {
		return "", err
	}

	ns, err := uuid.Parse(upgradeCode)
	if err != nil {
		return "", fmt.Errorf("mkmsi: cannot parse upgrade code %q: %w", upgradeCode, err)
	}

	return uuid.NewSHA1(ns, []byte(version+"/"+arch)).String(), nil
}

// packageCode derives the MSI PackageCode -- the summary information's
// "Revision number (UUID)" field -- for a version and architecture, the
// same way productCode derives the Product Id: a UUIDv5 in the
// UpgradeCode's namespace, so rebuilding the same commit reproduces the same
// value instead of getting whatever wixl randomizes it to.
//
// This is a deliberate departure from MSI convention, which calls for a
// unique PackageCode per built package file -- PackageCode exists to
// distinguish physical .msi files even when they install the same product,
// and normally two builds of identical bits still deserve two different
// PackageCodes because nothing guarantees the bits stayed identical. Making
// it deterministic is correct here specifically because the rest of this
// build has been made reproducible: once identical inputs are guaranteed to
// produce byte-identical output, a PackageCode that still changed on every
// rebuild would be actively misleading, not conservative.
//
// The hash input is distinguished from productCode's ("packagecode/" vs no
// prefix) so the two never collide: Windows must never see ProductCode and
// PackageCode take on the same GUID.
func packageCode(version, arch string) (string, error) {
	if _, err := msiVersion(version); err != nil {
		return "", err
	}

	ns, err := uuid.Parse(upgradeCode)
	if err != nil {
		return "", fmt.Errorf("mkmsi: cannot parse upgrade code %q: %w", upgradeCode, err)
	}

	return uuid.NewSHA1(ns, []byte("packagecode/"+version+"/"+arch)).String(), nil
}

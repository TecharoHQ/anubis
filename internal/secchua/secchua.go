// Package secchua is a Sec-Ch-Ua parser for Go programs. Given Sec-Ch-Ua header family values, it parses them into
// a [Client] structure with the tuples it parsed out.
//
// This is implemented against the ua-client-hints draft report: https://wicg.github.io/ua-client-hints/
package secchua

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrClientMismatch             = errors.New("secchua: Clients do not match")
	ErrClientVersionCountMismatch = errors.New("secchua: Client version count mismatch")
	ErrClientMobileMismatch       = errors.New("secchua: Client mobile flag mismatch")
	ErrClientPlatformMismatch     = errors.New("secchua: Client platform mismatch")
	ErrClientArchMismatch         = errors.New("secchua: Client architecture mismatch")
	ErrClientBitnessMismatch      = errors.New("secchua: Client bitness mismatch")
)

// Client is the information about a client that was parsed from the Sec-Ch-Ua header family.
//
// This is a _low-entropy_ hint, not an exact matcher. However in Anubis we will end up using this to confirm
// that one client is who they say they are when challenges require subrequests to be performed.
//
// Hopefully this makes scrapers have to adapt their shit.
type Client struct {
	Versions []Version `json:"versions"`
	Mobile   *bool     `json:"mobile"` // nil means unsure
	Platform string    `json:"platform"`
	Arch     string    `json:"arch"`
	Bitness  string    `json:"bitness"`
}

// Equals ensures that two parsed bits of client information match eachother.
//
// This assumes that Versions is sorted by Name.
func (lhs Client) Equals(rhs Client) error {
	var errs []error

	if len(lhs.Versions) != len(rhs.Versions) {
		errs = append(errs, fmt.Errorf("%w: want: %d, got: %d", ErrClientVersionCountMismatch, len(lhs.Versions), len(rhs.Versions)))
	} else {
		for i := range len(lhs.Versions) {
			if err := lhs.Versions[i].Equals(rhs.Versions[i]); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if (lhs.Mobile == nil) != (rhs.Mobile == nil) {
		errs = append(errs, ErrClientMobileMismatch)
	}

	if lhs.Mobile != nil && rhs.Mobile != nil {
		if *lhs.Mobile != *rhs.Mobile {
			errs = append(errs, fmt.Errorf("%w: want: %v, got: %v", ErrClientMobileMismatch, lhs.Mobile, rhs.Mobile))
		}
	}

	if lhs.Platform != rhs.Platform {
		errs = append(errs, fmt.Errorf("%w: want: %q, got: %q", ErrClientPlatformMismatch, lhs.Platform, rhs.Platform))
	}

	if lhs.Arch != rhs.Arch {
		errs = append(errs, fmt.Errorf("%w: want: %q, got: %q", ErrClientArchMismatch, lhs.Arch, rhs.Arch))
	}

	if lhs.Bitness != rhs.Bitness {
		errs = append(errs, fmt.Errorf("%w: want: %q, got: %q", ErrClientBitnessMismatch, lhs.Bitness, rhs.Bitness))
	}

	if len(errs) != 0 {
		return fmt.Errorf("%w: %w", ErrClientMismatch, errors.Join(errs...))
	}

	return nil
}

// Version is one brand/version pair from a client hint header.
//
// For example:
//
//	"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"
//
// This would parse to:
//
//	[]Version{
//	  {Name: "Not=A?Brand", Version: "99"},
//	  {Name: "Google Chrome", Version: "151"},
//	  {Name: "Chromium", Version: "151"},
//	}
type Version struct {
	Name    string `json:"name"`
	Version string `json:"major_version"`
}

// String formats a [Version] hint in a Sec-Ch-Ua header value.
func (v Version) String() string {
	return fmt.Sprintf("%q;v=%q", v.Name, v.Version)
}

var (
	ErrVersionMismatch = errors.New("secchua: version mismatch")
)

// Equals ensures that two Version hints match eachother.
//
// It returns an error if there are any mismatches.
func (lhs Version) Equals(rhs Version) error {
	// Skip Chrome sentinel strings
	if strings.Contains(rhs.Name, "Brand") && strings.Contains(lhs.Name, "Brand") {
		return nil
	}

	if (lhs.Name != rhs.Name) && (lhs.Version != rhs.Version) {
		return fmt.Errorf("%w: want: %s, got: %s", ErrVersionMismatch, lhs, rhs)
	}

	return nil
}

// ParseVersion parses the version fields in Sec-Ch-Ua and Sec-Ch-Ua-Full-Version-List headers.
func ParseVersion(input string) ([]Version, error) {
	var result []Version

	return result, nil
}

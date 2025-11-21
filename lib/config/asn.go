package config

import (
	"errors"
	"fmt"
)

var (
	ErrPrivateASN = errors.New("bot.ASNs: you have specified a private use ASN")
)

type ASNs struct {
	Match []uint32 `json:"match"`
}

func (a *ASNs) Valid() error {
	var errs []error

	for _, asn := range a.Match {
		if isPrivateASN(asn) {
			errs = append(errs, fmt.Errorf("%w: %d is private (see RFC 6996)", ErrPrivateASN, asn))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("bot.ASNs: invalid ASN settings: %w", errors.Join(errs...))
	}

	return nil
}

// isPrivateASN checks if an ASN is in the private use area.
//
// Based on RFC 6996 and IANA allocations.
func isPrivateASN(asn uint32) bool {
	switch {
	case asn >= 64512 && asn <= 65534:
		return true
	case asn >= 4200000000 && asn <= 4294967294:
		return true
	default:
		return false
	}
}

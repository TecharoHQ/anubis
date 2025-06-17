package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/cespare/xxhash/v2"
)

// SHA256sum computes a cryptographic hash. Still used for proof-of-work challenges
// where we need the security properties of a cryptographic hash function.
func SHA256sum(text string) string {
	hash := sha256.New()
	hash.Write([]byte(text))
	return hex.EncodeToString(hash.Sum(nil))
}

// FastHash is a high-performance non-cryptographic hash function suitable for
// internal caching, policy rule identification, and other performance-critical
// use cases where cryptographic security is not required.
func FastHash(text string) string {
	h := xxhash.Sum64String(text)
	return strconv.FormatUint(h, 16)
}

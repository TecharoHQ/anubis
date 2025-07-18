package fingerprint

import (
	"fmt"
	"net/http"
	"strings"
)

// JA4T represents a TCP fingerprint
type JA4T struct {
	Window      uint32
	Options     []uint8
	MSS         uint16
	WindowScale uint8
}

func (j JA4T) String() string {
	var sb strings.Builder

	// Start with the window size
	fmt.Fprintf(&sb, "%d", j.Window)

	// Append each option
	for i, opt := range j.Options {
		if i == 0 {
			fmt.Fprint(&sb, "_")
		} else {
			fmt.Fprint(&sb, "-")
		}
		fmt.Fprintf(&sb, "%d", opt)
	}

	// Append MSS and WindowScale
	fmt.Fprintf(&sb, "_%d_%d", j.MSS, j.WindowScale)

	return sb.String()
}

// GetTCPFingerprint extracts TCP fingerprint from HTTP request context
func GetTCPFingerprint(r *http.Request) *JA4T {
	ptr := r.Context().Value(tcpFingerprintKey{})
	if fpPtr, ok := ptr.(*JA4T); ok && ptr != nil && fpPtr != nil {
		return fpPtr
	}
	return nil
}

//go:build !linux && !freebsd

package fingerprint

import "net"

// AssignTCPFingerprint is not supported on this platform
func AssignTCPFingerprint(conn net.Conn) (*JA4T, error) {
	// Not supported on macOS and other platforms
	return &JA4T{}, nil
}

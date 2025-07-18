package fingerprint

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
)

// ApplyTLSFingerprinter configures a TLS server to capture TLS fingerprints
func ApplyTLSFingerprinter(server *http.Server) {
	if server.TLSConfig == nil {
		return
	}
	server.TLSConfig = server.TLSConfig.Clone()

	getConfigForClient := server.TLSConfig.GetConfigForClient

	if getConfigForClient == nil {
		getConfigForClient = func(info *tls.ClientHelloInfo) (*tls.Config, error) {
			return nil, nil
		}
	}

	server.TLSConfig.GetConfigForClient = func(clientHello *tls.ClientHelloInfo) (*tls.Config, error) {
		ja3n, ja4 := buildTLSFingerprint(clientHello)
		ptr := clientHello.Context().Value(tlsFingerprintKey{})
		if fpPtr, ok := ptr.(*TLSFingerprint); ok && ptr != nil && fpPtr != nil {
			fpPtr.ja3n.Store(&ja3n)
			fpPtr.ja4.Store(&ja4)
		}
		return getConfigForClient(clientHello)
	}
	server.ConnContext = func(ctx context.Context, c net.Conn) context.Context {
		ctx = context.WithValue(ctx, tlsFingerprintKey{}, &TLSFingerprint{})

		if tc, ok := c.(*tls.Conn); ok {
			tcpFP, err := AssignTCPFingerprint(tc.NetConn())
			if err == nil {
				ctx = context.WithValue(ctx, tcpFingerprintKey{}, tcpFP)
			} else {
				slog.Debug("ja4t error", "err", err)
			}
		}

		return ctx
	}
}

type tcpFingerprintKey struct{}
type tlsFingerprintKey struct{}

// TLSFingerprint represents TLS fingerprint data
type TLSFingerprint struct {
	ja3n atomic.Pointer[TLSFingerprintJA3N]
	ja4  atomic.Pointer[TLSFingerprintJA4]
}

// JA3N returns the JA3N fingerprint
func (f *TLSFingerprint) JA3N() *TLSFingerprintJA3N {
	return f.ja3n.Load()
}

// JA4 returns the JA4 fingerprint
func (f *TLSFingerprint) JA4() *TLSFingerprintJA4 {
	return f.ja4.Load()
}

const greaseMask = 0x0F0F
const greaseValue = 0x0a0a

// TLS extension numbers
const (
	extensionServerName              uint16 = 0
	extensionStatusRequest           uint16 = 5
	extensionSupportedCurves         uint16 = 10 // supported_groups in TLS 1.3, see RFC 8446, Section 4.2.7
	extensionSupportedPoints         uint16 = 11
	extensionSignatureAlgorithms     uint16 = 13
	extensionALPN                    uint16 = 16
	extensionSCT                     uint16 = 18
	extensionExtendedMasterSecret    uint16 = 23
	extensionSessionTicket           uint16 = 35
	extensionPreSharedKey            uint16 = 41
	extensionEarlyData               uint16 = 42
	extensionSupportedVersions       uint16 = 43
	extensionCookie                  uint16 = 44
	extensionPSKModes                uint16 = 45
	extensionCertificateAuthorities  uint16 = 47
	extensionSignatureAlgorithmsCert uint16 = 50
	extensionKeyShare                uint16 = 51
	extensionQUICTransportParameters uint16 = 57
	extensionRenegotiationInfo       uint16 = 0xff01
	extensionECHOuterExtensions      uint16 = 0xfd00
	extensionEncryptedClientHello    uint16 = 0xfe0d
)

func buildTLSFingerprint(hello *tls.ClientHelloInfo) (TLSFingerprintJA3N, TLSFingerprintJA4) {
	return TLSFingerprintJA3N(buildJA3N(hello, true)), buildJA4(hello)
}

// GetTLSFingerprint extracts TLS fingerprint from HTTP request context
func GetTLSFingerprint(r *http.Request) *TLSFingerprint {
	ptr := r.Context().Value(tlsFingerprintKey{})
	if fpPtr, ok := ptr.(*TLSFingerprint); ok && ptr != nil && fpPtr != nil {
		return fpPtr
	}
	return nil
}

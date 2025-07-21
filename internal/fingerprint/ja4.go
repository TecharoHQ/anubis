package fingerprint

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// TLSFingerprintJA4 represents a JA4 fingerprint
type TLSFingerprintJA4 struct {
	A [10]byte
	B [6]byte
	C [6]byte
}

func (f *TLSFingerprintJA4) String() string {
	if f == nil {
		return ""
	}

	return strings.Join([]string{
		string(f.A[:]),
		hex.EncodeToString(f.B[:]),
		hex.EncodeToString(f.C[:]),
	}, "_")
}

func buildJA4(hello *tls.ClientHelloInfo) (ja4 TLSFingerprintJA4) {
	buf := make([]byte, 0, 36)

	hasQuic := false

	for _, ext := range hello.Extensions {
		if ext == extensionQUICTransportParameters {
			hasQuic = true
		}
	}

	switch hasQuic {
	case true:
		buf = append(buf, 'q')
	case false:
		buf = append(buf, 't')
	}

	{
		var sslVersion uint16
		for _, v := range hello.SupportedVersions {
			if v&greaseMask != greaseValue {
				if v > sslVersion {
					sslVersion = v
				}
			}
		}

		switch sslVersion {
		case tls.VersionTLS10:
			buf = append(buf, '1', '0')
		case tls.VersionTLS11:
			buf = append(buf, '1', '1')
		case tls.VersionTLS12:
			buf = append(buf, '1', '2')
		case tls.VersionTLS13:
			buf = append(buf, '1', '3')
		default:
			sslVersion -= 0x0201
			buf = strconv.AppendUint(buf, uint64(sslVersion>>8), 10)
			buf = strconv.AppendUint(buf, uint64(sslVersion&0xff), 10)
		}

	}

	if slices.Contains(hello.Extensions, extensionServerName) && hello.ServerName != "" {
		buf = append(buf, 'd')
	} else {
		buf = append(buf, 'i')
	}

	ciphers := make([]uint16, 0, len(hello.CipherSuites))
	for _, cipher := range hello.CipherSuites {
		if cipher&greaseMask != greaseValue {
			ciphers = append(ciphers, cipher)
		}
	}

	extensionCount := 0
	extensions := make([]uint16, 0, len(hello.Extensions))
	for _, extension := range hello.Extensions {
		if extension&greaseMask != greaseValue {
			extensionCount++
			if extension != extensionALPN && extension != extensionServerName {
				extensions = append(extensions, extension)
			}
		}
	}

	schemes := make([]tls.SignatureScheme, 0, len(hello.SignatureSchemes))

	for _, scheme := range hello.SignatureSchemes {
		if scheme&greaseMask != greaseValue {
			schemes = append(schemes, scheme)
		}
	}

	//TODO: maybe little endian
	slices.Sort(ciphers)
	slices.Sort(extensions)
	//slices.Sort(schemes)

	if len(ciphers) < 10 {
		buf = append(buf, '0')
		buf = strconv.AppendUint(buf, uint64(len(ciphers)), 10)
	} else if len(ciphers) > 99 {
		buf = append(buf, '9', '9')
	} else {
		buf = strconv.AppendUint(buf, uint64(len(ciphers)), 10)
	}

	if extensionCount < 10 {
		buf = append(buf, '0')
		buf = strconv.AppendUint(buf, uint64(extensionCount), 10)
	} else if extensionCount > 99 {
		buf = append(buf, '9', '9')
	} else {
		buf = strconv.AppendUint(buf, uint64(extensionCount), 10)
	}

	if len(hello.SupportedProtos) > 0 && len(hello.SupportedProtos[0]) > 1 {
		buf = append(buf, hello.SupportedProtos[0][0], hello.SupportedProtos[0][len(hello.SupportedProtos[0])-1])
	} else {
		buf = append(buf, '0', '0')
	}

	copy(ja4.A[:], buf)

	ja4.B = ja4SHA256(uint16SliceToHex(ciphers))

	extBuf := uint16SliceToHex(extensions)

	if len(schemes) > 0 {
		extBuf = append(extBuf, '_')
		extBuf = append(extBuf, uint16SliceToHex(schemes)...)
	}

	ja4.C = ja4SHA256(extBuf)

	return ja4
}

func uint16SliceToHex[T ~uint16](in []T) (out []byte) {
	if len(in) == 0 {
		return out
	}
	out = slices.Grow(out, hex.EncodedLen(len(in)*2)+len(in))

	for _, n := range in {
		out = append(out, fmt.Sprintf("%04x", uint16(n))...)
		out = append(out, ',')
	}
	out = out[:len(out)-1]

	return out
}

func ja4SHA256(buf []byte) [6]byte {
	if len(buf) == 0 {
		return [6]byte{0, 0, 0, 0, 0, 0}
	}
	sum := sha256.Sum256(buf)

	return [6]byte(sum[:6])
}

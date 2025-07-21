package fingerprint

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"slices"
	"strconv"
)

// TLSFingerprintJA3N represents a JA3N fingerprint
type TLSFingerprintJA3N [md5.Size]byte

func (f TLSFingerprintJA3N) String() string {
	return hex.EncodeToString(f[:])
}

func buildJA3N(hello *tls.ClientHelloInfo, sortExtensions bool) TLSFingerprintJA3N {
	buf := make([]byte, 0, 256)

	{
		var sslVersion uint16
		var hasGrease bool
		for _, v := range hello.SupportedVersions {
			if v&greaseMask != greaseValue {
				if v > sslVersion {
					sslVersion = v
				}
			} else {
				hasGrease = true
			}
		}

		// maximum TLS 1.2 as specified on JA3, as TLS 1.3 is put in SupportedVersions
		if slices.Contains(hello.Extensions, extensionSupportedVersions) && hasGrease && sslVersion > tls.VersionTLS12 {
			sslVersion = tls.VersionTLS12
		}

		buf = strconv.AppendUint(buf, uint64(sslVersion), 10)
		buf = append(buf, ',')
	}

	n := 0
	for _, cipher := range hello.CipherSuites {
		//if !slices.Contains(greaseValues[:], cipher) {
		if cipher&greaseMask != greaseValue {
			buf = strconv.AppendUint(buf, uint64(cipher), 10)
			buf = append(buf, '-')
			n = 1
		}
	}

	buf = buf[:len(buf)-n]
	buf = append(buf, ',')
	n = 0

	extensions := hello.Extensions
	if sortExtensions {
		extensions = slices.Clone(extensions)
		slices.Sort(extensions)
	}

	for _, extension := range extensions {
		if extension&greaseMask != greaseValue {
			buf = strconv.AppendUint(buf, uint64(extension), 10)
			buf = append(buf, '-')
			n = 1
		}
	}

	buf = buf[:len(buf)-n]
	buf = append(buf, ',')
	n = 0

	for _, curve := range hello.SupportedCurves {
		if curve&greaseMask != greaseValue {
			buf = strconv.AppendUint(buf, uint64(curve), 10)
			buf = append(buf, '-')
			n = 1
		}
	}

	buf = buf[:len(buf)-n]
	buf = append(buf, ',')
	n = 0

	for _, point := range hello.SupportedPoints {
		buf = strconv.AppendUint(buf, uint64(point), 10)
		buf = append(buf, '-')
		n = 1
	}

	buf = buf[:len(buf)-n]

	sum := md5.Sum(buf)
	return TLSFingerprintJA3N(sum[:])
}

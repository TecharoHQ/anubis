package servicesid

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

// serviceSIDPrefix is the identifier authority Windows files per-service SIDs
// under. Everything below it is derived from the service name.
const serviceSIDPrefix = "S-1-5-80"

// anubisServiceName is the name the MSI registers the service under. It is the
// input to the SID derivation below, so it has to match ServiceInstall/@Name in
// run/windows/anubis.wxs exactly, case aside.
const AnubisServiceName = "Anubis"

// anubisServiceSID is the SID of the NT SERVICE\Anubis virtual account the
// service runs as, spelled out rather than computed so that the value the
// installer grants access to is visible in a diff and pinned by a test.
const AnubisServiceSID = "S-1-5-80-765274699-3418405142-632509039-2036741013-1444054785"

// Encode returns the SID of the NT SERVICE account belonging to a Windows
// service, in the string form icacls and the rest of Windows accept.
//
// Windows derives these rather than allocating them: it uppercases the service
// name, encodes it as UTF-16LE, takes the SHA-1 of that, and appends the digest
// to S-1-5-80 as five little-endian uint32 subauthorities. The service does not
// have to exist for the SID to be meaningful, which is what lets the installer
// grant access to the config directory before the service is registered.
//
// The SHA-1 here is Windows' choice of construction, not a security decision of
// ours. Any other hash would produce a SID that names nothing.
//
// This lives outside service_windows.go on purpose. Nothing in the build or in
// CI runs Windows, so anything behind that build constraint is compiled and
// never executed; keeping the arithmetic portable is what makes it testable.
func Encode(name string) string {
	encoded := utf16.Encode([]rune(strings.ToUpper(name)))

	buf := make([]byte, 2*len(encoded))
	for i, unit := range encoded {
		binary.LittleEndian.PutUint16(buf[2*i:], unit)
	}

	sum := sha1.Sum(buf)

	var sb strings.Builder
	sb.WriteString(serviceSIDPrefix)
	for i := 0; i < len(sum); i += 4 {
		fmt.Fprintf(&sb, "-%d", binary.LittleEndian.Uint32(sum[i:]))
	}

	return sb.String()
}

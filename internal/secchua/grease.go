package secchua

import "strings"

// IsGREASE reports whether a brand name is an intentionally meaningless padding brand.
func IsGREASE(name string) bool {
	var sb strings.Builder
	sb.Grow(len(name))

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			sb.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			sb.WriteRune(r + ('a' - 'A')) // convert to lowercase with ascii math
		case r >= '0' && r <= '9':
			sb.WriteRune(r)
		}
	}

	return sb.String() == "notabrand"
}

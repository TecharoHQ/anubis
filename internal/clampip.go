package internal

import "net/netip"

func ClampIP(addr netip.Addr) (netip.Prefix, bool) {
	switch {
	case addr.Is4():
		result, err := addr.Prefix(24)
		if err != nil {
			return netip.Prefix{}, false
		}
		return result, true

	case addr.Is4In6():
		// Extract the IPv4 address from IPv4-mapped IPv6 and clamp it
		ipv4 := addr.Unmap()
		result, err := ipv4.Prefix(24)
		if err != nil {
			return netip.Prefix{}, false
		}
		return result, true

	case addr.Is6():
		result, err := addr.Prefix(48)
		if err != nil {
			return netip.Prefix{}, false
		}
		return result, true

	default:
		return netip.Prefix{}, false
	}
}

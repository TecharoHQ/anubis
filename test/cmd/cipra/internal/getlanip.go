package internal

import (
	"fmt"
	"net"
)

// GetLANIP returns the first non-loopback IPv4 LAN IP address.
func GetLANIP() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		// Skip down or loopback interfaces
		if iface.Flags&(net.FlagUp|net.FlagLoopback) != net.FlagUp {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue // skip interfaces we can't query
		}

		for _, addr := range addrs {
			var ip net.IP

			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ip = ip.To4()
			if ip == nil {
				continue // not an IPv4 address
			}

			return ip, nil
		}
	}

	return nil, fmt.Errorf("no connected LAN IPv4 address found")
}

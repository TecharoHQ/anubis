package dnsbl

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

//go:generate go tool golang.org/x/tools/cmd/stringer -type=DroneBLResponse

type DroneBLResponse byte

const (
	AllGood               DroneBLResponse = 0
	IRCDrone              DroneBLResponse = 3
	Bottler               DroneBLResponse = 5
	UnknownSpambotOrDrone DroneBLResponse = 6
	DDOSDrone             DroneBLResponse = 7
	SOCKSProxy            DroneBLResponse = 8
	HTTPProxy             DroneBLResponse = 9
	ProxyChain            DroneBLResponse = 10
	OpenProxy             DroneBLResponse = 11
	OpenDNSResolver       DroneBLResponse = 12
	BruteForceAttackers   DroneBLResponse = 13
	OpenWingateProxy      DroneBLResponse = 14
	CompromisedRouter     DroneBLResponse = 15
	AutoRootingWorms      DroneBLResponse = 16
	AutoDetectedBotIP     DroneBLResponse = 17
	Unknown               DroneBLResponse = 255
)

func Reverse(ip net.IP) string {
	if ip.To4() != nil {
		return reverse4(ip)
	}

	return reverse6(ip)
}

func reverse4(ip net.IP) string {
	splitAddress := strings.Split(ip.String(), ".")

	// swap first and last octet
	splitAddress[0], splitAddress[3] = splitAddress[3], splitAddress[0]
	// swap middle octets
	splitAddress[1], splitAddress[2] = splitAddress[2], splitAddress[1]

	return strings.Join(splitAddress, ".")
}

func reverse6(ip net.IP) string {
	ipBytes := []byte(ip)
	var sb strings.Builder

	for i := len(ipBytes) - 1; i >= 0; i-- {
		// Split the byte into two nibbles
		highNibble := ipBytes[i] >> 4
		lowNibble := ipBytes[i] & 0x0F

		// Append the nibbles in reversed order
		sb.WriteString(fmt.Sprintf("%x.%x.", lowNibble, highNibble))
	}

	return sb.String()[:len(sb.String())-1]
}

func Lookup(ipStr string) (DroneBLResponse, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return Unknown, errors.New("dnsbl: input is not an IP address")
	}

	revIP := Reverse(ip) + ".dnsbl.dronebl.org"

	ips, err := net.LookupIP(revIP)
	if err != nil {
		var dnserr *net.DNSError
		if errors.As(err, &dnserr) {
			if dnserr.IsNotFound {
				return AllGood, nil
			}
		}

		return Unknown, err
	}

	if len(ips) != 0 {
		for _, ip := range ips {
			return DroneBLResponse(ip.To4()[3]), nil
		}
	}

	return UnknownSpambotOrDrone, nil
}

package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
)

// FetchBlocklist reads the blocklist over HTTP and returns every non-commented
// line parsed as an IP address in CIDR notation. IPv4 addresses are returned as
// /32, IPv6 addresses as /128.
//
// This function was generated with GLM 4.7.
func FetchBlocklist(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	var lines []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines and comments (lines starting with #)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		addr, err := netip.ParseAddr(line)
		if err != nil {
			// Skip lines that aren't valid IP addresses
			continue
		}

		var cidr string
		if addr.Is4() {
			cidr = fmt.Sprintf("%s/32", addr.String())
		} else {
			cidr = fmt.Sprintf("%s/128", addr.String())
		}
		lines = append(lines, cidr)
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}

	return lines, nil
}

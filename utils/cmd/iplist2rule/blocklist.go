package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"

	"github.com/TecharoHQ/anubis/internal/iplist"
)

// ParseBlocklist reads a plain-text blocklist and returns every non-commented
// line parsed as an IP address in CIDR notation. IPv4 addresses are returned as
// /32, IPv6 addresses as /128.
//
// This function was generated with GLM 4.7.
func ParseBlocklist(list io.Reader) ([]string, error) {
	var addrs []string

	scanner := bufio.NewScanner(list)
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
		addrs = append(addrs, cidr)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return addrs, nil
}

// FetchBlocklist fetches url over HTTP and parses the response body as a
// blocklist. JSON responses (detected via the Content-Type header or a
// ".json" URL suffix) are parsed with ParsePrefixList; everything else is
// treated as a plain-text list and parsed with ParseBlocklist.
func FetchBlocklist(url string) ([]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	if isJSONBlocklist(url, resp.Header.Get("Content-Type")) {
		return ParsePrefixList(resp.Body)
	}

	return ParseBlocklist(resp.Body)
}

func isJSONBlocklist(url, contentType string) bool {
	if contentType == "json" {
		return true
	}

	return strings.HasSuffix(strings.ToLower(url), ".json")
}

// ParsePrefixList decodes a JSON document read from list in the Google/OpenAI
// bot IP range format and returns every IPv4 and IPv6 prefix it contains.
func ParsePrefixList(list io.Reader) ([]string, error) {
	pl, err := iplist.Parse(list)
	if err != nil {
		return nil, err
	}
	return pl.CIDRs(), nil
}

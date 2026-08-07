package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
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
	if contentType ==  "json" {
		return true
	}

	return strings.HasSuffix(strings.ToLower(url), ".json")
}

type Prefix struct {
	IPv4Prefix string `json:"ipv4Prefix"`
	IPv6Prefix string `json:"ipv6Prefix"`
}

type PrefixList struct {
	CreationTime string   `json:"creationTime"`
	Prefixes     []Prefix `json:"prefixes"`
}

// ParsePrefixList decodes a JSON document read from list in the Google/OpenAI
// bot IP range format and returns every IPv4 and IPv6 prefix it contains.
func ParsePrefixList(list io.Reader) ([]string, error) {
	var pl PrefixList
	if err := json.NewDecoder(list).Decode(&pl); err != nil {
		return nil, fmt.Errorf("can't decode prefix list: %w", err)
	}

	var prefixes []string
	for _, p := range pl.Prefixes {
		switch {
		case p.IPv4Prefix != "":
			prefixes = append(prefixes, p.IPv4Prefix)
		case p.IPv6Prefix != "":
			prefixes = append(prefixes, p.IPv6Prefix)
		}
	}

	return prefixes, nil
}

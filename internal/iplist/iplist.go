// Package iplist parses published bot IP prefix lists in the Google/OpenAI JSON format.
package iplist

import (
	"encoding/json"
	"fmt"
	"io"
)

// Prefix is a single IPv4 or IPv6 CIDR advertised by a bot provider.
type Prefix struct {
	IPv4Prefix string `json:"ipv4Prefix"`
	IPv6Prefix string `json:"ipv6Prefix"`
}

// PrefixList is a published list of bot IP prefixes.
type PrefixList struct {
	CreationTime string   `json:"creationTime"`
	Prefixes     []Prefix `json:"prefixes"`
}

// Parse decodes a JSON document in the Google/OpenAI bot IP range format.
func Parse(r io.Reader) (*PrefixList, error) {
	var pl PrefixList
	if err := json.NewDecoder(r).Decode(&pl); err != nil {
		return nil, fmt.Errorf("can't decode prefix list: %w", err)
	}
	return &pl, nil
}

// CIDRs returns every IPv4 and IPv6 prefix in the list.
func (pl *PrefixList) CIDRs() []string {
	if pl == nil {
		return nil
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
	return prefixes
}

package dns

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"slices"
	"strings"
)

var (
	DNSLookupAddr = net.LookupAddr
	DNSLookupHost = net.LookupHost
)

type Dns struct {
	cache *DnsCache
	ctx   context.Context
}

func New(ctx context.Context, cache *DnsCache) *Dns {
	return &Dns{
		cache: cache,
		ctx:   ctx,
	}
}

// ReverseDNS performs a reverse DNS lookup for the given IP address and trims the trailing dot from the results.
func (d *Dns) ReverseDNS(addr string) ([]string, error) {
	slog.Debug("DNS: performing reverse lookup", "addr", addr)

	if cached, ok := d.getCachedReverse(addr); ok {
		return cached, nil
	}

	names, err := DNSLookupAddr(addr)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			slog.Debug("DNS: no PTR record found", "addr", addr)
			return []string{}, nil
		}
		slog.Error("DNS: reverse lookup failed", "addr", addr, "err", err)
		return nil, err
	}

	slog.Debug("DNS: reverse lookup successful", "addr", addr, "names", names)

	trimmedNames := make([]string, len(names))
	for i, name := range names {
		trimmedNames[i] = strings.TrimSuffix(name, ".")
	}
	d.reverseCachePut(addr, trimmedNames)

	return trimmedNames, nil
}

// LookupHost performs a forward DNS lookup for the given hostname.
func (d *Dns) LookupHost(host string) ([]string, error) {
	slog.Debug("DNS: performing forward lookup", "host", host)

	if cached, ok := d.getCachedForward(host); ok {
		return cached, nil
	}

	addrs, err := DNSLookupHost(host)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			slog.Debug("DNS: no A/AAAA record found", "host", host)
			return []string{}, nil
		}
		slog.Error("DNS: forward lookup failed", "host", host, "err", err)
		return nil, err
	}

	slog.Debug("DNS: forward lookup successful", "host", host, "addrs", addrs)
	d.forwardCachePut(host, addrs)
	return addrs, nil
}

// verifyFCrDNSInternal performs the second half of the FCrDNS check, using a
// pre-fetched list of names to perform the forward lookups.
func (d *Dns) verifyFCrDNSInternal(addr string, names []string) bool {
	for _, name := range names {
		if cached, err := d.LookupHost(name); err == nil {
			if slices.Contains(cached, addr) {
				slog.Info("DNS: forward lookup confirmed original IP", "name", name, "addr", addr)
				return true
			}
			continue
		}
	}

	slog.Info("DNS: could not confirm original IP in forward lookups", "addr", addr)
	return false
}

// VerifyFCrDNS performs a forward-confirmed reverse DNS (FCrDNS) lookup for the given IP address,
// optionally matching against a provided pattern.
func (d *Dns) VerifyFCrDNS(addr string, pattern *string) bool {
	if pattern != nil {
		slog.Debug("DNS: performing FCrDNS lookup", "addr", addr, "pattern", *pattern)
	} else {
		slog.Debug("DNS: performing FCrDNS lookup", "addr", addr, "pattern", "")
	}

	var names []string
	if names, _ = d.ReverseDNS(addr); len(names) == 0 {
		return false
	}

	// If a pattern is provided, check for a match.
	if pattern != nil {
		anyNameMatched := false
		for _, name := range names {
			matched, err := regexp.MatchString(*pattern, name)
			if err != nil {
				slog.Error("DNS: verifyFCrDNS invalid regex pattern", "err", err)
				return false // Invalid pattern is a failure.
			}
			if matched {
				anyNameMatched = true
				break
			}
		}

		if !anyNameMatched {
			slog.Debug("DNS: FCrDNS no PTR matches the pattern", "addr", addr, "pattern", *pattern)
			return false
		}
		slog.Debug("DNS: FCrDNS PTR matched pattern, proceeding with forward check", "addr", addr, "pattern", *pattern)
	}

	// If we're here, either there was no pattern, or the pattern matched.
	// Proceed with the forward lookup confirmation.
	return d.verifyFCrDNSInternal(addr, names)
}

// ArpaReverseIP performs translation from ip v4/v6 to arpa reverse notation
func (d *Dns) ArpaReverseIP(addr string) (string, error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return addr, errors.New("invalid IP address")
	}

	if ipv4 := ip.To4(); ipv4 != nil {
		return fmt.Sprintf("%d.%d.%d.%d", ipv4[3], ipv4[2], ipv4[1], ipv4[0]), nil
	}

	ipv6 := ip.To16()
	if ipv6 == nil {
		return addr, errors.New("invalid IPv6 address")
	}

	hexBytes := make([]byte, hex.EncodedLen(len(ipv6)))
	hex.Encode(hexBytes, ipv6)

	var sb strings.Builder
	sb.Grow(len(hexBytes)*2 - 1)

	for i := len(hexBytes) - 1; i >= 0; i-- {
		sb.WriteByte(hexBytes[i])
		if i > 0 {
			sb.WriteByte('.')
		}
	}
	return sb.String(), nil
}

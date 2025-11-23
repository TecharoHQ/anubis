package internal

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	DNSLookupAddr = net.LookupAddr
	DNSLookupHost = net.LookupHost
)

type DnsResult struct {
	entries    []string
	expiration time.Time
}

type Dns struct {
	forwardCache map[string]DnsResult
	reverseCache map[string]DnsResult
	forwardTTL   int
	reverseTTL   int
}

func NewDNS(forwardTTL int, reverseTTL int) *Dns {
	return &Dns{
		forwardCache: map[string]DnsResult{},
		reverseCache: map[string]DnsResult{},
		forwardTTL:   forwardTTL,
		reverseTTL:   reverseTTL,
	}
}

func (d *Dns) getCachedForward(host string) ([]string, bool) {
	if cached, ok := d.forwardCache[host]; ok {
		if time.Now().Before(cached.expiration) {
			return cached.entries, true
		}
		delete(d.forwardCache, host)
	}
	return nil, false
}

func (d *Dns) getCachedReverse(addr string) ([]string, bool) {
	if cached, ok := d.reverseCache[addr]; ok {
		if time.Now().Before(cached.expiration) {
			return cached.entries, true
		}
		delete(d.forwardCache, addr)
	}
	return nil, false
}

func (d *Dns) forwardCachePut(host string, entries []string) {
	d.forwardCache[host] = DnsResult{
		entries:    entries,
		expiration: time.Now().Add(time.Duration(d.forwardTTL) * time.Second),
	}
}

func (d *Dns) reverseCachePut(addr string, entries []string) {
	d.reverseCache[addr] = DnsResult{
		entries:    entries,
		expiration: time.Now().Add(time.Duration(d.reverseTTL) * time.Second),
	}
}

// lookupAddrAndTrim performs a reverse DNS lookup and trims the trailing dot from the results.
func (d *Dns) lookupAddrAndTrim(addr string) ([]string, error) {
	names, err := DNSLookupAddr(addr)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			slog.Debug("lookupAddrAndTrim: no PTR record found", "addr", addr)
			return []string{}, nil
		}
		return nil, err
	}

	for i, name := range names {
		names[i] = strings.TrimSuffix(name, ".")
	}

	return names, nil
}

// verifyFCrDNSInternal performs the second half of the FCrDNS check, using a
// pre-fetched list of names to perform the forward lookups.
func (d *Dns) verifyFCrDNSInternal(addr string, names []string) bool {
	for _, name := range names {
		if cached, ok := d.getCachedForward(name); ok {
			slog.Debug("verifyFCrDNS: forward lookup cache hit", "name", name)
			if slices.Contains(cached, addr) {
				slog.Info("verifyFCrDNS: forward lookup confirmed original IP", "name", name, "addr", addr)
				return true
			}
			continue
		}

		slog.Debug("verifyFCrDNS: forward lookup cache miss", "name", name)
		ips, err := DNSLookupHost(name)
		if err != nil {
			if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
				slog.Debug("verifyFCrDNS: no A/AAAA record found", "name", name)
				continue
			}
			slog.Error("verifyFCrDNS: forward lookup failed", "name", name, "err", err)
			continue
		}
		d.forwardCachePut(name, ips)
		slog.Debug("verifyFCrDNS: forward lookup found", "name", name, "ips", ips)

		if slices.Contains(ips, addr) {
			slog.Info("verifyFCrDNS: forward lookup confirmed original IP", "name", name, "addr", addr)
			return true
		}
	}

	slog.Info("verifyFCrDNS: could not confirm original IP in forward lookups", "addr", addr)
	return false
}

// ReverseDNS performs a reverse DNS lookup for the given IP address.
func (d *Dns) ReverseDNS(addr string) ([]string, error) {
	if cached, ok := d.getCachedReverse(addr); ok {
		slog.Debug("reverseDNS lookup found in reverse cache", "addr", addr)
		return cached, nil
	}

	slog.Debug("performing reverseDNS lookup", "addr", addr)
	names, err := d.lookupAddrAndTrim(addr)
	if err != nil {
		slog.Error("reverseDNS lookup failed", "addr", addr, "err", err)
		return []string{}, err
	}

	d.reverseCachePut(addr, names)
	slog.Debug("reverseDNS lookup found", "addr", addr, "names", names)
	return names, nil
}

// LookupHost performs a forward DNS lookup for the given hostname.
func (d *Dns) LookupHost(host string) ([]string, error) {
	if cached, ok := d.getCachedForward(host); ok {
		slog.Debug("lookupHost found in forward cache", "host", host)
		return cached, nil
	}

	slog.Debug("performing lookupHost", "host", host)
	addrs, err := DNSLookupHost(host)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			slog.Debug("lookupHost: no A/AAAA record found", "host", host)
			return []string{}, nil
		}
		slog.Error("lookupHost failed", "host", host, "err", err)
		return []string{}, err
	}

	d.forwardCachePut(host, addrs)
	slog.Debug("lookupHost found", "host", host, "addrs", addrs)
	return addrs, nil
}

// VerifyFCrDNS performs a forward-confirmed reverse DNS (FCrDNS) lookup for the given IP address,
// optionally matching against a provided pattern.
func (d *Dns) VerifyFCrDNS(addr string, pattern *string) bool {
	var names []string
	if cached, ok := d.getCachedReverse(addr); ok {
		slog.Debug("verifyFCrDNS: reverse lookup cache hit", "addr", addr)
		names = cached
	} else {
		slog.Debug("verifyFCrDNS: reverse lookup cache miss", "addr", addr)
		var err error
		names, err = d.lookupAddrAndTrim(addr)
		if err != nil {
			slog.Error("verifyFCrDNS: reverse lookup failed", "addr", addr, "err", err)
			return false
		}
		d.reverseCachePut(addr, names)
	}
	slog.Debug("verifyFCrDNS: reverse lookup found", "addr", addr, "names", names)

	// If a pattern is provided, check for a match.
	if pattern != nil {
		anyNameMatched := false
		for _, name := range names {
			matched, err := regexp.MatchString(*pattern, name)
			if err != nil {
				slog.Error("verifyFCrDNS: invalid regex pattern", "err", err)
				return false // Invalid pattern is a failure.
			}
			if matched {
				anyNameMatched = true
				break
			}
		}

		if !anyNameMatched {
			slog.Debug("verifyFCrDNS: reverse lookup did not match pattern", "addr", addr, "pattern", *pattern)
			return false
		}
		slog.Debug("verifyFCrDNS: reverse lookup matched pattern, proceeding with forward check", "addr", addr, "pattern", *pattern)
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

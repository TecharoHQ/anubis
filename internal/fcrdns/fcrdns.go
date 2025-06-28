package fcrdns

import (
	"context"
	"net"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/decaymap"
)

type FCrDNS struct {
	resolver             *net.Resolver
	forwardLookupCacheV4 *decaymap.Impl[string, []netip.Addr]
	forwardLookupCacheV6 *decaymap.Impl[string, []netip.Addr]
	reverseLookupCache   *decaymap.Impl[string, []string]
}

func NewFCrDNS() *FCrDNS {
	return &FCrDNS{
		resolver:             &net.Resolver{},
		forwardLookupCacheV4: decaymap.New[string, []netip.Addr](),
		forwardLookupCacheV6: decaymap.New[string, []netip.Addr](),
		reverseLookupCache:   decaymap.New[string, []string](),
	}
}

func (f *FCrDNS) Check(ip string, allowedDomainRegex *regexp.Regexp) (bool, error) {
	clientAddr, err := netip.ParseAddr(ip)
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hosts, err := f.reverseLookup(ctx, ip)
	if err != nil {
		return false, err
	}

	for _, host := range hosts {
		if !allowedDomainRegex.MatchString(host) {
			continue
		}

		addresses, err := f.forwardLookup(ctx, host, clientAddr.Is6())
		if err != nil {
			return false, err
		}
		for _, addr := range addresses {
			if addr == clientAddr {
				return true, nil
			}
		}
	}

	return false, nil
}

func (f *FCrDNS) Cleanup() {
	f.forwardLookupCacheV4.Cleanup()
	f.forwardLookupCacheV6.Cleanup()
	f.reverseLookupCache.Cleanup()
}

func (f *FCrDNS) reverseLookup(ctx context.Context, addr string) ([]string, error) {
	if result, ok := f.reverseLookupCache.Get(addr); ok {
		return result, nil
	}

	rawHosts, err := f.resolver.LookupAddr(ctx, addr)
	if err != nil {
		return []string{}, err
	}

	hosts := []string{}
	for _, host := range rawHosts {
		hosts = append(hosts, strings.TrimSuffix(host, "."))
	}

	f.reverseLookupCache.Set(addr, hosts, time.Hour)
	return hosts, nil
}

func (f *FCrDNS) forwardLookup(ctx context.Context, host string, ipv6 bool) ([]netip.Addr, error) {
	cache := f.forwardLookupCacheV4
	network := "ip4"
	if ipv6 {
		cache = f.forwardLookupCacheV6
		network = "ip6"
	}

	if result, ok := cache.Get(host); ok {
		return result, nil
	}

	result, err := f.resolver.LookupNetIP(ctx, network, host)
	if err != nil {
		return []netip.Addr{}, err
	}

	cache.Set(host, result, time.Hour)
	return result, nil
}

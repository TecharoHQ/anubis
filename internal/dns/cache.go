package dns

import (
	"log/slog"
	"time"

	"github.com/TecharoHQ/anubis/lib/store"

	_ "github.com/TecharoHQ/anubis/lib/store/all"
)

type DnsCache struct {
	forward    store.JSON[[]string]
	reverse    store.JSON[[]string]
	forwardTTL time.Duration
	reverseTTL time.Duration
}

func NewDNSCache(forwardTTL int, reverseTTL int, backend store.Interface) *DnsCache {
	return &DnsCache{
		forward: store.JSON[[]string]{
			Underlying: backend,
			Prefix:     "forwardDNS",
		},
		reverse: store.JSON[[]string]{
			Underlying: backend,
			Prefix:     "reverseDNS",
		},
		forwardTTL: time.Duration(forwardTTL) * time.Second,
		reverseTTL: time.Duration(reverseTTL) * time.Second,
	}
}

func (d *Dns) getCachedForward(host string) ([]string, bool) {
	if cached, err := d.cache.forward.Get(d.ctx, host); err == nil {
		slog.Debug("DNS: forward cache hit", "name", host, "ips", cached)
		return cached, true
	}
	slog.Debug("DNS: forward cache miss", "name", host)
	return nil, false
}

func (d *Dns) getCachedReverse(addr string) ([]string, bool) {
	if cached, err := d.cache.reverse.Get(d.ctx, addr); err == nil {
		slog.Debug("DNS: reverse cache hit", "addr", addr, "names", cached)
		return cached, true
	}
	slog.Debug("DNS: reverse cache miss", "addr", addr)
	return nil, false
}

func (d *Dns) forwardCachePut(host string, entries []string) {
	d.cache.forward.Set(d.ctx, host, entries, d.cache.forwardTTL)
}

func (d *Dns) reverseCachePut(addr string, entries []string) {
	d.cache.reverse.Set(d.ctx, addr, entries, d.cache.reverseTTL)
}

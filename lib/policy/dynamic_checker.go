package policy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/iplist"
	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	"github.com/gaissmai/bart"
)

const (
	dynamicRefreshInterval = 24 * time.Hour
	dynamicRetryInterval   = 15 * time.Minute
	dynamicMaxRetries      = 5
	dynamicHTTPTimeout     = 30 * time.Second
	dynamicMaxBodyBytes    = 8 << 20
)

// DynamicRemoteAddrChecker matches client IPs against a prefix list fetched
// from a remote JSON document. The list starts empty and is replaced on each
// successful refresh. Failed fetches never clobber a previously loaded list.
type DynamicRemoteAddrChecker struct {
	url    string
	logger *slog.Logger
	client *http.Client

	mu           sync.RWMutex
	prefixTable  *bart.Lite
	hash         string
	creationTime string
	rebuildCount int

	cycleStart      time.Time
	consecutiveFail int

	refreshInterval time.Duration
	retryInterval   time.Duration
	maxRetries      int

	stopped chan struct{}
}

func NewDynamicRemoteAddrChecker(ctx context.Context, rawURL string, logger *slog.Logger) (checker.Impl, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if err := validateDynamicURL(rawURL); err != nil {
		return nil, err
	}

	c := &DynamicRemoteAddrChecker{
		url:             rawURL,
		logger:          logger,
		client:          &http.Client{Timeout: dynamicHTTPTimeout},
		prefixTable:     new(bart.Lite),
		hash:            internal.FastHash(""),
		refreshInterval: dynamicRefreshInterval,
		retryInterval:   dynamicRetryInterval,
		maxRetries:      dynamicMaxRetries,
		stopped:         make(chan struct{}),
	}

	go c.run(ctx)
	return c, nil
}

func validateDynamicURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%w: %q", config.ErrInvalidDynamicRemoteAddrURL, rawURL)
	}
	return nil
}

func (c *DynamicRemoteAddrChecker) Check(r *http.Request) (bool, error) {
	host := r.Header.Get("X-Real-IP")
	if host == "" {
		return false, fmt.Errorf("%w: header X-Real-IP is not set", ErrMisconfiguration)
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false, fmt.Errorf("%w: %s is not an IP address: %w", ErrMisconfiguration, host, err)
	}

	if addr.Is6() && addr.Is4In6() {
		addr = addr.Unmap()
	}

	c.mu.RLock()
	table := c.prefixTable
	c.mu.RUnlock()

	if table == nil {
		return false, nil
	}

	return table.Contains(addr), nil
}

func (c *DynamicRemoteAddrChecker) Hash() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hash
}

func (c *DynamicRemoteAddrChecker) run(ctx context.Context) {
	defer close(c.stopped)

	for {
		if ctx.Err() != nil {
			return
		}

		delay := c.refreshOnce(ctx)
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (c *DynamicRemoteAddrChecker) refreshOnce(ctx context.Context) time.Duration {
	if c.cycleStart.IsZero() {
		c.cycleStart = time.Now()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return c.handleRetryable(ctx, "can't build dynamic remote address list request", err)
	}
	req.Header.Set("User-Agent", "TecharoHQ-Anubis/"+anubis.Version)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0
		}
		return c.handleRetryable(ctx, "can't fetch dynamic remote address list", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	switch {
	case resp.StatusCode >= 500:
		return c.handleRetryable(ctx, "dynamic remote address list fetch returned server error", fmt.Errorf("http status %d", resp.StatusCode))
	case resp.StatusCode >= 400:
		c.logger.WarnContext(ctx, "dynamic remote address list fetch returned client error", "url", c.url, "status", resp.StatusCode)
		return c.delayUntilNextCycle()
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return c.handleRetryable(ctx, "dynamic remote address list fetch returned unexpected status", fmt.Errorf("http status %d", resp.StatusCode))
	}

	pl, err := iplist.Parse(io.LimitReader(resp.Body, dynamicMaxBodyBytes))
	if err != nil {
		c.logger.WarnContext(ctx, "failed to parse dynamic remote address list", "url", c.url, "err", err)
		return c.delayUntilNextCycle()
	}

	if err := c.applyList(pl); err != nil {
		c.logger.WarnContext(ctx, "failed to apply dynamic remote address list", "url", c.url, "err", err)
		return c.delayUntilNextCycle()
	}

	c.cycleStart = time.Now()
	c.consecutiveFail = 0
	return c.refreshInterval
}

func (c *DynamicRemoteAddrChecker) applyList(pl *iplist.PrefixList) error {
	if pl.CreationTime != "" && pl.CreationTime == c.creationTime {
		c.logger.Debug("dynamic remote address list unchanged, skip rebuild", "url", c.url, "creationTime", pl.CreationTime)
		return nil
	}

	cidrs := pl.CIDRs()
	table := new(bart.Lite)
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		table.Insert(prefix)
	}

	hash := internal.FastHash(strings.Join(cidrs, ","))

	c.mu.Lock()
	c.prefixTable = table
	c.hash = hash
	c.creationTime = pl.CreationTime
	c.rebuildCount++
	c.mu.Unlock()

	c.logger.Info("updated dynamic remote address list", "url", c.url, "prefixes", len(cidrs), "creationTime", pl.CreationTime)
	return nil
}

func (c *DynamicRemoteAddrChecker) handleRetryable(ctx context.Context, msg string, err error) time.Duration {
	c.consecutiveFail++
	if c.consecutiveFail < c.maxRetries {
		c.logger.WarnContext(ctx, msg, "url", c.url, "attempt", c.consecutiveFail, "err", err)
		return c.retryInterval
	}

	c.logger.WarnContext(ctx, "dynamic remote address list fetch failed, giving up until next refresh window", "url", c.url, "attempts", c.consecutiveFail, "err", err)
	return c.delayUntilNextCycle()
}

func (c *DynamicRemoteAddrChecker) delayUntilNextCycle() time.Duration {
	c.consecutiveFail = 0
	remaining := time.Until(c.cycleStart.Add(c.refreshInterval))
	if remaining < 0 {
		remaining = 0
	}
	c.cycleStart = time.Time{}
	return remaining
}

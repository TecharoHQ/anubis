package ogtags

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"syscall"
)

// GetOGTags is the main function that retrieves Open Graph tags for a URL
func (c *OGTagCache) GetOGTags(ctx context.Context, url *url.URL, originalHost string) (map[string]string, error) {
	if url == nil {
		return nil, errors.New("nil URL provided, cannot fetch OG tags")
	}

	if len(c.ogOverride) != 0 {
		return c.ogOverride, nil
	}

	target := c.getTarget(url)
	cacheKey := c.generateCacheKey(target, originalHost)

	// Check cache first
	if cachedTags := c.checkCache(ctx, cacheKey); cachedTags != nil {
		return cachedTags, nil
	}

	// Fetch HTML content, passing the original host
	doc, err := c.fetchHTMLDocumentWithCache(ctx, target, originalHost, cacheKey)
	if errors.Is(err, syscall.ECONNREFUSED) {
		slog.Debug("Connection refused, returning empty tags")
		return nil, nil
	} else if errors.Is(err, ErrOgHandled) {
		// Error was handled in fetchHTMLDocument, return empty tags
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Extract OG tags
	ogTags := c.extractOGTags(doc)

	// Store in cache
	c.cache.Set(ctx, cacheKey, ogTags, c.ogTimeToLive)

	return ogTags, nil
}

func (c *OGTagCache) generateCacheKey(target string, originalHost string) string {
	var cacheKey string

	if c.ogCacheConsiderHost {
		cacheKey = target + "|" + originalHost
	} else {
		cacheKey = target
	}
	return cacheKey
}

// checkCache checks if we have the tags cached and returns them if so
func (c *OGTagCache) checkCache(ctx context.Context, cacheKey string) map[string]string {
	if cachedTags, err := c.cache.Get(ctx, cacheKey); err == nil {
		slog.Debug("cache hit", "tags", cachedTags)
		return cachedTags
	}
	slog.Debug("cache miss", "url", cacheKey)
	return nil
}

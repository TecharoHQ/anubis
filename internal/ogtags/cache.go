package ogtags

import (
	"errors"
	"log/slog"
	"net/url"
	"syscall"
)

// GetOGTags is the main function that retrieves Open Graph tags for a URL
func (c *OGTagCache) GetOGTags(url *url.URL) (map[string]string, error) {
	if url == nil {
		return nil, errors.New("nil URL provided, cannot fetch OG tags")
	}
	urlStr := c.getTarget(url)
	// Check cache first
	if cachedTags := c.checkCache(urlStr); cachedTags != nil {
		return cachedTags, nil
	}

	// Fetch HTML content
	doc, err := c.fetchHTMLDocument(urlStr)
	if errors.Is(err, syscall.ECONNREFUSED) {
		slog.Debug("Connection refused, returning empty tags")
		return nil, nil
	} else if errors.Is(err, ErrNotFound) {
		// not even worth a debug log...
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Extract OG tags
	ogTags := c.extractOGTags(doc)

	// Store in cache
	c.cache.Set(urlStr, ogTags, c.ogTimeToLive)

	return ogTags, nil
}

// checkCache checks if we have the tags cached and returns them if so
func (c *OGTagCache) checkCache(urlStr string) map[string]string {
	if cachedTags, ok := c.cache.Get(urlStr); ok {
		slog.Debug("cache hit", "tags", cachedTags)
		return cachedTags
	}
	slog.Debug("cache miss", "url", urlStr)
	return nil
}

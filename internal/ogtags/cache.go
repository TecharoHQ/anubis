package ogtags

import (
	"log/slog"
	"net/url"
)

// GetOGTags is the main function that retrieves Open Graph tags for a URL
func (c *OGTagCache) GetOGTags(url *url.URL) (map[string]string, error) {
	urlStr := c.getTarget(url)
	// Check cache first
	if cachedTags := c.checkCache(urlStr); cachedTags != nil {
		return cachedTags, nil
	}

	// Fetch HTML content
	doc, err := c.fetchHTMLDocument(urlStr)
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
		slog.Debug("GetOGTags", "cache", "hit", "tags", cachedTags)
		return cachedTags
	}
	slog.Debug("GetOGTags", "cache", "miss", "url", urlStr)
	return nil
}

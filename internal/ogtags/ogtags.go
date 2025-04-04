package ogtags

import (
	"net/http"
	"net/url"
	"time"

	"github.com/TecharoHQ/anubis/decaymap"
)

type OGTagCache struct {
	cache            *decaymap.Impl[string, map[string]string]
	target           string
	ogPassthrough    bool
	ogTimeToLive     time.Duration
	approvedTags     []string
	approvedPrefixes []string
	client           *http.Client
	maxContentLength int64
}

func NewOGTagCache(target string, ogPassthrough bool, ogTimeToLive time.Duration) *OGTagCache {
	// Predefined approved tags and prefixes
	// In the future, these could come from configuration
	defaultApprovedTags := []string{"description", "keywords", "author"}
	defaultApprovedPrefixes := []string{"og:", "twitter:", "fediverse:"}
	client := &http.Client{
		Timeout: 5 * time.Second, /*make this configurable?*/
	}

	const maxContentLength = 16 << 20 // 16 MiB in bytes

	return &OGTagCache{
		cache:            decaymap.New[string, map[string]string](),
		target:           target,
		ogPassthrough:    ogPassthrough,
		ogTimeToLive:     ogTimeToLive,
		approvedTags:     defaultApprovedTags,
		approvedPrefixes: defaultApprovedPrefixes,
		client:           client,
		maxContentLength: maxContentLength,
	}
}

func (c *OGTagCache) getTarget(u *url.URL) string {
	return c.target + u.Path
}

func (c *OGTagCache) Cleanup() {
	c.cache.Cleanup()
}

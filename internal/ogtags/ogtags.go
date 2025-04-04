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
}

func NewOGTagCache(target string, ogPassthrough bool, ogTimeToLive time.Duration) *OGTagCache {
	// Predefined approved tags and prefixes
	// In the future, these could come from configuration
	defaultApprovedTags := []string{"description"}
	defaultApprovedPrefixes := []string{"og:", "twitter:", "fediverse:"}
	client := &http.Client{
		Timeout: 10 * time.Second, /*todo: make this configurable*/
	}
	return &OGTagCache{
		cache:            decaymap.New[string, map[string]string](),
		target:           target,
		ogPassthrough:    ogPassthrough,
		ogTimeToLive:     ogTimeToLive,
		approvedTags:     defaultApprovedTags,
		approvedPrefixes: defaultApprovedPrefixes,
		client:           client,
	}
}

func (c *OGTagCache) getTarget(u *url.URL) string {
	return c.target + u.Path
}

func (c *OGTagCache) Cleanup() {
	c.cache.Cleanup()
}

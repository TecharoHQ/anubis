package ogtags

import (
	"net/http"
	"net/url"
	"time"

	"github.com/TecharoHQ/anubis/decaymap"
)

var client *http.Client

func init() {
	client = &http.Client{
		Timeout: 10 * time.Second, /*todo: make this configurable*/
	}
}

type OGTagCache struct {
	cache           *decaymap.Impl[string, map[string]string]
	target          string
	ogPassthrough   bool
	ogTimeToLive    time.Duration
	ogQueryDistinct bool
}

func NewOGTagCache(target string, ogPassthrough bool, ogTimeToLive time.Duration, ogQueryDistinct bool) *OGTagCache {
	//if target == "" {
	// slog.Error("NewOGTagCache: target is empty. OpenGraph support is disabled.")
	// return nil
	//}
	return &OGTagCache{
		cache:           decaymap.New[string, map[string]string](),
		target:          target,
		ogPassthrough:   ogPassthrough,
		ogTimeToLive:    ogTimeToLive,
		ogQueryDistinct: ogQueryDistinct,
	}
}

func (c *OGTagCache) getTarget(url *url.URL) string {
	// fixme: nil pointer here
	if c.ogQueryDistinct && url.RawQuery != "" {
		return c.target + url.Path + "?" + url.RawQuery
	}
	return c.target + url.Path
}

func (c *OGTagCache) Cleanup() {
	c.cache.Cleanup()
}

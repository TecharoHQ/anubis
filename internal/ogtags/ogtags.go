package ogtags

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/decaymap"
	"golang.org/x/net/html"
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
	if target == "" {
		slog.Error("NewOGTagCache: target is empty. OpenGraph support is disabled.")
		return nil
	}
	return &OGTagCache{
		cache:           decaymap.New[string, map[string]string](),
		target:          target,
		ogPassthrough:   ogPassthrough,
		ogTimeToLive:    ogTimeToLive,
		ogQueryDistinct: ogQueryDistinct,
	}
}

func (c *OGTagCache) getTarget(url *url.URL) string {
	if c.ogQueryDistinct && url.RawQuery != "" {
		return c.target + url.Path + "?" + url.RawQuery
	}
	return c.target + url.Path
}

func (c *OGTagCache) GetOGTags(url *url.URL) (map[string]string, error) {
	urlStr := c.getTarget(url)
	slog.Info("GetOGTags", "url", urlStr)
	if cachedTags, ok := c.cache.Get(urlStr); ok {
		slog.Info("GetOGTags", "cache", "hit", "tags", cachedTags)
		return cachedTags, nil
	}
	slog.Info("GetOGTags", "cache", "miss", "url", urlStr)

	resp, err := client.Get(urlStr) // todo: remove useless logs, refactor this method
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	ogTags := make(map[string]string)
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			var property, content string
			for _, attr := range n.Attr {
				if (attr.Key == "property" || attr.Key == "name") && strings.HasPrefix(attr.Val, "og:") {
					property = attr.Val
				}
				if attr.Key == "content" {
					content = attr.Val
				}
			}
			if property != "" && content != "" {
				ogTags[property] = content
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	c.cache.Set(urlStr, ogTags, c.ogTimeToLive)
	return ogTags, nil
}

func (c *OGTagCache) Cleanup() {
	c.cache.Cleanup()
}

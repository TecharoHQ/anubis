package lib

import (
	"net/http"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/decaymap"
	"golang.org/x/net/html"
)

type OGTagCache struct {
	cache *decaymap.Impl[string, map[string]string]
	ttl   time.Duration
}

func NewOGTagCache(ttl time.Duration) *OGTagCache {
	return &OGTagCache{
		cache: decaymap.New[string, map[string]string](),
		ttl:   ttl,
	}
}

func (c *OGTagCache) GetOGTags(url string) (map[string]string, error) {
	if cachedTags, ok := c.cache.Get(url); ok {
		return cachedTags, nil
	}

	resp, err := http.Get(url)
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
				if attr.Key == "property" && strings.HasPrefix(attr.Val, "og:") {
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

	c.cache.Set(url, ogTags, c.ttl)
	return ogTags, nil
}

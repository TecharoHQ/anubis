package ogtags

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/decaymap"
)

const (
	maxContentLength = 16 << 20        // 16 MiB in bytes, if there is a reasonable reason that you need more than this...Why?
	httpTimeout      = 5 * time.Second /*todo: make this configurable?*/
)

type OGTagCache struct {
	cache               *decaymap.Impl[string, map[string]string]
	targetURL           *url.URL
	ogCacheConsiderHost bool
	ogPassthrough       bool
	ogTimeToLive        time.Duration
	approvedTags        []string
	approvedPrefixes    []string
	client              *http.Client
}

func NewOGTagCache(target string, ogPassthrough bool, ogTimeToLive time.Duration, ogTagsConsiderHost bool) *OGTagCache {
	// Predefined approved tags and prefixes
	// In the future, these could come from configuration
	defaultApprovedTags := []string{"description", "keywords", "author"}
	defaultApprovedPrefixes := []string{"og:", "twitter:", "fediverse:"}

	var parsedTargetURL *url.URL
	var err error

	if target == "" {
		// Default to localhost if target is empty
		parsedTargetURL, _ = url.Parse("http://localhost")
	} else {
		parsedTargetURL, err = url.Parse(target)
		if err != nil {
			slog.Debug("og: failed to parse target URL, treating as non-unix", "target", target, "error", err)
			// If parsing fails, treat it as a non-unix target for backward compatibility or default behavior
			// For now, assume it's not a scheme issue but maybe an invalid char, etc.
			// A simple string target might be intended if it's not a full URL.
			parsedTargetURL = &url.URL{Scheme: "http", Host: target} // Assume http if scheme missing and host-like
			if !strings.Contains(target, "://") && !strings.HasPrefix(target, "unix:") {
				// If it looks like just a host/host:port (and not unix), prepend http:// (todo: is this bad...? Trace path to see if i can yell at user to do it right)
				parsedTargetURL, _ = url.Parse("http://" + target) // fetch cares about scheme but anubis doesn't
			}
		}
	}

	client := &http.Client{
		Timeout: httpTimeout,
	}

	// Configure custom transport for Unix sockets
	if parsedTargetURL.Scheme == "unix" {
		socketPath := parsedTargetURL.Path // For unix scheme, path is the socket path
		client.Transport = &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		}
	}

	return &OGTagCache{
		cache:               decaymap.New[string, map[string]string](),
		targetURL:           parsedTargetURL, // Store the parsed URL
		ogPassthrough:       ogPassthrough,
		ogTimeToLive:        ogTimeToLive,
		ogCacheConsiderHost: ogTagsConsiderHost, // todo: refactor to be a separate struct
		approvedTags:        defaultApprovedTags,
		approvedPrefixes:    defaultApprovedPrefixes,
		client:              client,
	}
}

// getTarget constructs the target URL string for fetching OG tags.
// For Unix sockets, it creates a "fake" HTTP URL that the custom dialer understands.
func (c *OGTagCache) getTarget(u *url.URL) string {
	if c.targetURL.Scheme == "unix" {
		// The custom dialer ignores the host, but we need a valid http URL structure.
		// Use "unix" as a placeholder host. Path and Query from original request are appended.
		fakeURL := &url.URL{
			Scheme:   "http", // Scheme must be http/https for client.Get
			Host:     "unix", // Arbitrary host, ignored by custom dialer
			Path:     u.Path,
			RawQuery: u.RawQuery,
		}
		return fakeURL.String()
	}

	// For regular http/https targets
	target := *c.targetURL // Make a copy
	target.Path = u.Path
	target.RawQuery = u.RawQuery
	return target.String()

}

func (c *OGTagCache) Cleanup() {
	if c.cache != nil {
		c.cache.Cleanup()
	}
}

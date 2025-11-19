package ogtags

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store"
)

const (
	maxContentLength = 8 << 20         // 8 MiB is enough for anyone
	httpTimeout      = 5 * time.Second /*todo: make this configurable?*/

	schemeSeparatorLength = 3 // Length of "://"
	querySeparatorLength  = 1 // Length of "?" for query strings
)

type OGTagCache struct {
	ogOverride map[string]string
	targetURL  *url.URL
	client     *http.Client
	transport  *http.Transport
	cache      store.JSON[map[string]string]

	// Pre-built strings for optimization
	unixPrefix          string // "http://unix"
	targetSNI           string
	targetHost          string
	approvedPrefixes    []string
	approvedTags        []string
	ogTimeToLive        time.Duration
	ogPassthrough       bool
	ogCacheConsiderHost bool
	targetSNIAuto       bool
	insecureSkipVerify  bool
	sniClients          map[string]*http.Client
	transportMu         sync.RWMutex
}

type TargetOptions struct {
	Host               string
	SNI                string
	InsecureSkipVerify bool
}

func NewOGTagCache(target string, conf config.OpenGraph, backend store.Interface, targetOpts TargetOptions) *OGTagCache {
	// Predefined approved tags and prefixes
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

	transport := http.DefaultTransport.(*http.Transport).Clone()

	// Configure custom transport for Unix sockets
	if parsedTargetURL.Scheme == "unix" {
		socketPath := parsedTargetURL.Path // For unix scheme, path is the socket path
		transport.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		}
	}

	targetSNIAuto := targetOpts.SNI == "auto"

	if targetOpts.SNI != "" && !targetSNIAuto {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.ServerName = targetOpts.SNI
	}

	if targetOpts.InsecureSkipVerify {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	client := &http.Client{
		Timeout:   httpTimeout,
		Transport: transport,
	}

	return &OGTagCache{
		cache: store.JSON[map[string]string]{
			Underlying: backend,
			Prefix:     "ogtags:",
		},
		targetURL:           parsedTargetURL,
		ogPassthrough:       conf.Enabled,
		ogTimeToLive:        conf.TimeToLive,
		ogCacheConsiderHost: conf.ConsiderHost,
		ogOverride:          conf.Override,
		approvedTags:        defaultApprovedTags,
		approvedPrefixes:    defaultApprovedPrefixes,
		client:              client,
		transport:           transport,
		unixPrefix:          "http://unix",
		targetHost:          targetOpts.Host,
		targetSNI:           targetOpts.SNI,
		targetSNIAuto:       targetSNIAuto,
		insecureSkipVerify:  targetOpts.InsecureSkipVerify,
		sniClients:          make(map[string]*http.Client),
	}
}

// getTarget constructs the target URL string for fetching OG tags.
// Optimized to minimize allocations by building strings directly.
func (c *OGTagCache) getTarget(u *url.URL) string {
	var escapedPath = u.EscapedPath() // will cause an allocation if path contains special characters
	if c.targetURL.Scheme == "unix" {
		// Build URL string directly without creating intermediate URL object
		var sb strings.Builder
		sb.Grow(len(c.unixPrefix) + len(escapedPath) + len(u.RawQuery) + querySeparatorLength) // Pre-allocate
		sb.WriteString(c.unixPrefix)
		sb.WriteString(escapedPath)
		if u.RawQuery != "" {
			sb.WriteByte('?')
			sb.WriteString(u.RawQuery)
		}
		return sb.String()
	}

	// For regular http/https targets, build URL string directly
	var sb strings.Builder
	// Pre-calculate size: scheme + "://" + host + path + "?" + query
	estimatedSize := len(c.targetURL.Scheme) + schemeSeparatorLength + len(c.targetURL.Host) + len(escapedPath) + len(u.RawQuery) + querySeparatorLength
	sb.Grow(estimatedSize)

	sb.WriteString(c.targetURL.Scheme)
	sb.WriteString("://")
	sb.WriteString(c.targetURL.Host)
	sb.WriteString(escapedPath)
	if u.RawQuery != "" {
		sb.WriteByte('?')
		sb.WriteString(u.RawQuery)
	}

	return sb.String()
}

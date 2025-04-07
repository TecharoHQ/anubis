package ogtags

import (
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"log/slog"
	"mime"
	"net"
	"net/http"
)

var (
	ErrNotFound = errors.New("page not found") /*todo: refactor into common errors lib? */
	emptyMap    = map[string]string{}          // used to indicate an empty result in the cache. Can't use nil as it would be a cache miss.
)

func (c *OGTagCache) fetchHTMLDocument(urlStr string) (*html.Node, error) {
	resp, err := c.client.Get(urlStr)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			slog.Debug("og: request timed out", "url", urlStr)
			c.cache.Set(urlStr, emptyMap, c.ogTimeToLive/2) // Cache empty result for half the TTL to not spam the server
		}
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	// this defer will call MaxBytesReader's Close, which closes the original body.
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Debug("og: received non-OK status code", "url", urlStr, "status", resp.StatusCode)
		c.cache.Set(urlStr, emptyMap, c.ogTimeToLive) // Cache empty result for non-successful status codes
		return nil, ErrNotFound
	}

	// Check content type
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		// assume non html body
		return nil, fmt.Errorf("missing Content-Type header")
	} else {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil {
			// Malformed Content-Type header
			return nil, fmt.Errorf("invalid Content-Type '%s': %w", ct, err)
		}

		if mediaType != "text/html" && mediaType != "application/xhtml+xml" {
			return nil, fmt.Errorf("unsupported Content-Type: %s", mediaType)
		}
	}

	resp.Body = http.MaxBytesReader(nil, resp.Body, c.maxContentLength)

	doc, err := html.Parse(resp.Body)
	if err != nil {
		// Check if the error is specifically because the limit was exceeded
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			slog.Debug("og: content exceeded max length", "url", urlStr, "limit", c.maxContentLength)
			return nil, fmt.Errorf("content too large: exceeded %d bytes", c.maxContentLength)
		}
		// parsing error (e.g., malformed HTML)
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return doc, nil
}

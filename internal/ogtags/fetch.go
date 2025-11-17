package ogtags

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"

	"golang.org/x/net/html"
)

var (
	ErrOgHandled = errors.New("og: handled error") // used to indicate that the error was handled and should not be logged
	emptyMap     = map[string]string{}             // used to indicate an empty result in the cache. Can't use nil as it would be a cache miss.
)

// fetchHTMLDocumentWithCache fetches the HTML document from the given URL string,
// preserving the original host header.
func (c *OGTagCache) fetchHTMLDocumentWithCache(ctx context.Context, urlStr string, originalHost string, cacheKey string) (*html.Node, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	// Set the Host header to the original host
	if originalHost != "" {
		req.Host = originalHost
	}

	// Add proxy headers
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("User-Agent", "Anubis-OGTag-Fetcher/1.0") // For tracking purposes

	// Send the request
	resp, err := c.client.Do(req)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			slog.Debug("og: request timed out", "url", urlStr)
			c.cache.Set(ctx, cacheKey, emptyMap, c.ogTimeToLive/2) // Cache empty result for half the TTL to not spam the server
		}
		return nil, fmt.Errorf("http get failed: %w", err)
	}

	// Ensure the response body is closed
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Debug("og: error closing response body", "url", urlStr, "error", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		slog.Debug("og: received non-OK status code", "url", urlStr, "status", resp.StatusCode)
		c.cache.Set(ctx, cacheKey, emptyMap, c.ogTimeToLive) // Cache empty result for non-successful status codes
		return nil, fmt.Errorf("%w: page not found", ErrOgHandled)
	}

	// Check content type
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		return nil, fmt.Errorf("missing Content-Type header")
	} else {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil {
			slog.Debug("og: malformed Content-Type header", "url", urlStr, "contentType", ct)
			return nil, fmt.Errorf("%w malformed Content-Type header: %w", ErrOgHandled, err)
		}

		if mediaType != "text/html" && mediaType != "application/xhtml+xml" {
			slog.Debug("og: unsupported Content-Type", "url", urlStr, "contentType", mediaType)
			return nil, fmt.Errorf("%w unsupported Content-Type: %s", ErrOgHandled, mediaType)
		}
	}

	resp.Body = http.MaxBytesReader(nil, resp.Body, maxContentLength)

	doc, err := html.Parse(resp.Body)
	if err != nil {
		// Check if the error is specifically because the limit was exceeded
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			slog.Debug("og: content exceeded max length", "url", urlStr, "limit", maxContentLength)
			return nil, fmt.Errorf("content too large: exceeded %d bytes", maxContentLength)
		}
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return doc, nil
}

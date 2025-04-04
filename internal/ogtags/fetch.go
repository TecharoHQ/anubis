package ogtags

import (
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
)

var ErrNotFound = errors.New("page not found") /*todo: refactor into common errors lib? */

// fetchHTMLDocument fetches and parses the HTML document
func (c *OGTagCache) fetchHTMLDocument(urlStr string) (*html.Node, error) {
	resp, err := c.client.Get(urlStr)
	if os.IsTimeout(err) {
		c.cache.Set(urlStr, nil, c.ogTimeToLive/2) // Cache empty result for half the TTL to not spam the server
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrNotFound
	}

	// Check content type
	ct := resp.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return nil, fmt.Errorf("invalid Content-Type: %v", err)
	}

	if mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		return nil, fmt.Errorf("unsupported Content-Type: %s", mediaType)
	}

	// Check content length
	if clStr := resp.Header.Get("Content-Length"); clStr != "" {
		if cl, err := strconv.ParseInt(clStr, 10, 64); err == nil && cl > c.maxContentLength {
			return nil, fmt.Errorf("content too large: %d bytes", cl)
		}
	}

	// Limit reader in case Content-Length is missing or incorrect
	limitedReader := io.LimitReader(resp.Body, c.maxContentLength)

	// Parse HTML
	doc, err := html.Parse(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return doc, nil
}

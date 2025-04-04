package ogtags

import (
	"fmt"
	"net/http"

	"golang.org/x/net/html"
)

// fetchHTMLDocument fetches and parses the HTML document
func (c *OGTagCache) fetchHTMLDocument(urlStr string) (*html.Node, error) {
	resp, err := c.client.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch HTML document: status code %d", resp.StatusCode)
	}

	return html.Parse(resp.Body)
}

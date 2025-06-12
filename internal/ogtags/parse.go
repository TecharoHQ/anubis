package ogtags

import (
	"strings"

	"golang.org/x/net/html"
)

// extractOGTags traverses the HTML document and extracts approved Open Graph tags
func (c *OGTagCache) extractOGTags(doc *html.Node) map[string]string {
	ogTags := make(map[string]string)

	var traverseNodes func(*html.Node)
	traverseNodes = func(n *html.Node) {
		if isOGMetaTag(n) {
			property, content := c.extractMetaTagInfo(n)
			if property != "" {
				ogTags[property] = content
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			traverseNodes(child)
		}
	}

	traverseNodes(doc)
	return ogTags
}

// isOGMetaTag checks if a node is *any* meta tag
func isOGMetaTag(n *html.Node) bool {
	if n == nil {
		return false
	}
	return n.Type == html.ElementNode && n.Data == "meta"
}

// extractMetaTagInfo extracts property and content from a meta tag
func (c *OGTagCache) extractMetaTagInfo(n *html.Node) (property, content string) {
	var rawProperty string

	// Single pass through attributes, using range to avoid bounds checking
	for _, attr := range n.Attr {
		switch attr.Key {
		case "property", "name":
			rawProperty = attr.Val
		case "content":
			content = attr.Val
		}
		// Early exit if we have both
		if rawProperty != "" && content != "" {
			break
		}
	}

	if rawProperty == "" {
		return "", content
	}

	// Check prefixes first (more common case)
	for _, prefix := range c.approvedPrefixes {
		if strings.HasPrefix(rawProperty, prefix) {
			return rawProperty, content
		}
	}

	// Check exact matches
	for _, tag := range c.approvedTags {
		if rawProperty == tag {
			return rawProperty, content
		}
	}

	return "", content
}

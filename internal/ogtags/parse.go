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
		// isOGMetaTag only checks if it's a <meta> tag.
		// The actual filtering happens in extractMetaTagInfo now.
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
// *and* checks if the property is approved.
// Returns empty property string if the tag is not approved.
func (c *OGTagCache) extractMetaTagInfo(n *html.Node) (property, content string) {
	var rawProperty string // Store the property found before approval check

	for _, attr := range n.Attr {
		if attr.Key == "property" || attr.Key == "name" {
			rawProperty = attr.Val
		}
		if attr.Key == "content" {
			content = attr.Val
		}
	}

	// Check if the rawProperty is approved
	isApproved := false
	for _, prefix := range c.approvedPrefixes {
		if strings.HasPrefix(rawProperty, prefix) {
			isApproved = true
			break
		}
	}
	// Check exact approved tags if not already approved by prefix
	if !isApproved {
		for _, tag := range c.approvedTags {
			if rawProperty == tag {
				isApproved = true
				break
			}
		}
	}

	// Only return the property if it's approved
	if isApproved {
		property = rawProperty
	}

	// Content is returned regardless, but property will be "" if not approved
	return property, content
}

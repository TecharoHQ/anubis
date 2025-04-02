package ogtags

import (
	"strings"

	"golang.org/x/net/html"
)

// extractOGTags traverses the HTML document and extracts Open Graph tags
func (c *OGTagCache) extractOGTags(doc *html.Node) map[string]string {
	ogTags := make(map[string]string)

	var traverseNodes func(*html.Node)
	traverseNodes = func(n *html.Node) {
		if isOGMetaTag(n) {
			property, content := extractMetaTagInfo(n)
			if property != "" && content != "" {
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

// isOGMetaTag checks if a node is an Open Graph meta tag
func isOGMetaTag(n *html.Node) bool {
	return n.Type == html.ElementNode && n.Data == "meta"
}

// extractMetaTagInfo extracts property and content from a meta tag
func extractMetaTagInfo(n *html.Node) (property, content string) {
	for _, attr := range n.Attr {
		if (attr.Key == "property" || attr.Key == "name") && strings.HasPrefix(attr.Val, "og:") {
			property = attr.Val
		}
		if attr.Key == "content" {
			content = attr.Val
		}
	}
	return property, content
}

package ogtags

import (
	"strings"

	"golang.org/x/net/html"
)

// extractOGTags traverses the HTML document and extracts approved Open Graph tags
// Optimized to pre-allocate map and reduce allocations
func (c *OGTagCache) extractOGTags(doc *html.Node) map[string]string {
	// Pre-allocate map with reasonable capacity
	ogTags := make(map[string]string, 10)

	// Stack-based traversal to avoid function call overhead
	stack := make([]*html.Node, 0, 32)
	stack = append(stack, doc)

	for len(stack) > 0 {
		// Pop from stack
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Check if it's a meta tag using the function
		if isOGMetaTag(n) {
			property, content := c.extractMetaTagInfo(n)
			if property != "" {
				ogTags[property] = content
			}
		}

		// Add children to stack in reverse order
		for child := n.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}

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
// Optimized to reduce string operations
func (c *OGTagCache) extractMetaTagInfo(n *html.Node) (property, content string) {
	var rawProperty string

	// Single pass through attributes
	for i := 0; i < len(n.Attr); i++ {
		attr := &n.Attr[i]
		switch attr.Key {
		case "property", "name":
			rawProperty = attr.Val
		case "content":
			content = attr.Val
		}
	}

	if rawProperty == "" {
		return "", content
	}

	// Check prefixes first (more common case)
	for i := 0; i < len(c.approvedPrefixes); i++ {
		if strings.HasPrefix(rawProperty, c.approvedPrefixes[i]) {
			return rawProperty, content
		}
	}

	// Check exact matches
	for i := 0; i < len(c.approvedTags); i++ {
		if rawProperty == c.approvedTags[i] {
			return rawProperty, content
		}
	}

	return "", content
}

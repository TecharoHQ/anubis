package ogtags

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestExtractOGTags(t *testing.T) {
	tests := []struct {
		name     string
		htmlStr  string
		expected map[string]string
	}{
		{
			name: "Basic OG tags",
			htmlStr: `<!DOCTYPE html>
				<html>
				<head>
					<meta property="og:title" content="Test Title" />
					<meta property="og:description" content="Test Description" />
				</head>
				<body></body>
				</html>`,
			expected: map[string]string{
				"og:title":       "Test Title",
				"og:description": "Test Description",
			},
		},
		{
			name: "OG tags with name attribute",
			htmlStr: `<!DOCTYPE html>
				<html>
				<head>
					<meta name="og:title" content="Test Title" />
					<meta property="og:description" content="Test Description" />
				</head>
				<body></body>
				</html>`,
			expected: map[string]string{
				"og:title":       "Test Title",
				"og:description": "Test Description",
			},
		},
		{
			name: "No OG tags",
			htmlStr: `<!DOCTYPE html>
				<html>
				<head>
					<meta name="description" content="Test Description" />
				</head>
				<body></body>
				</html>`,
			expected: map[string]string{},
		},
		{
			name: "Empty content",
			htmlStr: `<!DOCTYPE html>
				<html>
				<head>
					<meta property="og:title" content="" />
					<meta property="og:description" content="Test Description" />
				</head>
				<body></body>
				</html>`,
			expected: map[string]string{
				"og:title":       "",
				"og:description": "Test Description",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(tt.htmlStr))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			cache := NewOGTagCache("", true, time.Minute, false)
			ogTags := cache.extractOGTags(doc)

			if !reflect.DeepEqual(ogTags, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, ogTags)
			}
		})
	}
}

func TestIsOGMetaTag(t *testing.T) {
	tests := []struct {
		name     string
		nodeHTML string
		expected bool
	}{
		{
			name:     "Meta OG tag",
			nodeHTML: `<meta property="og:title" content="Test">`,
			expected: true,
		},
		{
			name:     "Regular meta tag",
			nodeHTML: `<meta name="description" content="Test">`,
			expected: true, // This is still a meta tag, extractMetaTagInfo will filter
		},
		{
			name:     "Not a meta tag",
			nodeHTML: `<div>Test</div>`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(tt.nodeHTML))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			// Find the first element node (should be our test node)
			var node *html.Node
			var findMetaNode func(*html.Node)
			findMetaNode = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "meta" {
					node = n
					return
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					findMetaNode(c)
					if node != nil {
						return
					}
				}
			}
			findMetaNode(doc)

			result := isOGMetaTag(node)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractMetaTagInfo(t *testing.T) {
	tests := []struct {
		name             string
		nodeHTML         string
		expectedProperty string
		expectedContent  string
	}{
		{
			name:             "OG title with property",
			nodeHTML:         `<meta property="og:title" content="Test Title">`,
			expectedProperty: "og:title",
			expectedContent:  "Test Title",
		},
		{
			name:             "OG description with name",
			nodeHTML:         `<meta name="og:description" content="Test Description">`,
			expectedProperty: "og:description",
			expectedContent:  "Test Description",
		},
		{
			name:             "Regular meta tag",
			nodeHTML:         `<meta name="description" content="Test Description">`,
			expectedProperty: "",
			expectedContent:  "Test Description",
		},
		{
			name:             "No content",
			nodeHTML:         `<meta property="og:title">`,
			expectedProperty: "og:title",
			expectedContent:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(tt.nodeHTML))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			// Find the first element node (should be our test node)
			var node *html.Node
			var findMetaNode func(*html.Node)
			findMetaNode = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "meta" {
					node = n
					return
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					findMetaNode(c)
					if node != nil {
						return
					}
				}
			}
			findMetaNode(doc)

			property, content := extractMetaTagInfo(node)

			if property != tt.expectedProperty {
				t.Errorf("expected property %s, got %s", tt.expectedProperty, property)
			}

			if content != tt.expectedContent {
				t.Errorf("expected content %s, got %s", tt.expectedContent, content)
			}
		})
	}
}

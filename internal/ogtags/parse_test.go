package ogtags

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
	"golang.org/x/net/html"
)

// TestExtractOGTags updated with correct expectations based on filtering logic
func TestExtractOGTags(t *testing.T) {
	// Use a cache instance that reflects the default approved lists
	testCache := NewOGTagCache("", config.OpenGraph{
		Enabled:      false,
		ConsiderHost: false,
		TimeToLive:   time.Minute,
	}, memory.New(t.Context()), TargetOptions{})
	// Manually set approved tags/prefixes based on the user request for clarity
	testCache.approvedTags = []string{"description"}
	testCache.approvedPrefixes = []string{"og:"}

	tests := []struct {
		expected map[string]string
		name     string
		htmlStr  string
	}{
		{
			name: "Basic OG tags", // Includes standard 'description' meta tag
			htmlStr: `<!DOCTYPE html>
				<html>
				<head>
					<meta property="og:title" content="Test Title" />
					<meta property="og:description" content="Test Description" />
					<meta name="description" content="Regular Description" />
					<meta name="keywords" content="test, keyword" />
				</head>
				<body></body>
				</html>`,
			expected: map[string]string{
				"og:title":       "Test Title",
				"og:description": "Test Description",
				"description":    "Regular Description",
			},
		},
		{
			name: "OG tags with name attribute",
			htmlStr: `<!DOCTYPE html>
				<html>
				<head>
					<meta name="og:title" content="Test Title" />
					<meta property="og:description" content="Test Description" />
					<meta name="twitter:card" content="summary" />
				</head>
				<body></body>
				</html>`,
			expected: map[string]string{
				"og:title":       "Test Title",
				"og:description": "Test Description",
				// twitter:card is still not approved
			},
		},
		{
			name: "No approved OG tags", // Contains only standard 'description'
			htmlStr: `<!DOCTYPE html>
				<html>
				<head>
					<meta name="description" content="Test Description" />
					<meta name="keywords" content="Test" />
				</head>
				<body></body>
				</html>`,
			expected: map[string]string{
				"description": "Test Description",
			},
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
		{
			name: "Explicitly approved tag",
			htmlStr: `<!DOCTYPE html>
						<html>
						<head>
							<meta property="description" content="Approved Description Tag" />
						</head>
						<body></body>
						</html>`,
			expected: map[string]string{
				// This is approved because "description" is in cache.approvedTags
				"description": "Approved Description Tag",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := html.Parse(strings.NewReader(tt.htmlStr))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			ogTags := testCache.extractOGTags(doc)

			if !reflect.DeepEqual(ogTags, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, ogTags)
			}
		})
	}
}

func TestIsOGMetaTag(t *testing.T) {
	tests := []struct {
		name       string
		nodeHTML   string
		targetNode string // Helper to find the right node in parsed fragment
		expected   bool
	}{
		{
			name:       "Meta OG tag",
			nodeHTML:   `<meta property="og:title" content="Test">`,
			targetNode: "meta",
			expected:   true,
		},
		{
			name:       "Regular meta tag",
			nodeHTML:   `<meta name="description" content="Test">`,
			targetNode: "meta",
			expected:   true,
		},
		{
			name:       "Not a meta tag",
			nodeHTML:   `<div>Test</div>`,
			targetNode: "div",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap the partial HTML in basic structure for parsing
			fullHTML := "<html><head>" + tt.nodeHTML + "</head><body></body></html>"
			doc, err := html.Parse(strings.NewReader(fullHTML))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			// Find the target element node (meta or div based on targetNode)
			var node *html.Node
			var findNode func(*html.Node)
			findNode = func(n *html.Node) {
				// Skip finding if already found
				if node != nil {
					return
				}
				// Check if current node matches type and tag data
				if n.Type == html.ElementNode && n.Data == tt.targetNode {
					node = n
					return
				}
				// Recursively check children
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					findNode(c)
				}
			}
			findNode(doc) // Start search from root

			if node == nil {
				t.Fatalf("Could not find target node '%s' in test HTML", tt.targetNode)
			}

			// Call the function under test
			result := isOGMetaTag(node)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractMetaTagInfo(t *testing.T) {
	// Use a cache instance that reflects the default approved lists
	testCache := NewOGTagCache("", config.OpenGraph{
		Enabled:      false,
		ConsiderHost: false,
		TimeToLive:   time.Minute,
	}, memory.New(t.Context()), TargetOptions{})
	testCache.approvedTags = []string{"description"}
	testCache.approvedPrefixes = []string{"og:"}

	tests := []struct {
		name             string
		nodeHTML         string
		expectedProperty string
		expectedContent  string
	}{
		{
			name:             "OG title with property (approved by prefix)",
			nodeHTML:         `<meta property="og:title" content="Test Title">`,
			expectedProperty: "og:title",
			expectedContent:  "Test Title",
		},
		{
			name:             "OG description with name (approved by prefix)",
			nodeHTML:         `<meta name="og:description" content="Test Description">`,
			expectedProperty: "og:description",
			expectedContent:  "Test Description",
		},
		{
			name:             "Regular meta tag (name=description, approved by exact match)", // Updated name for clarity
			nodeHTML:         `<meta name="description" content="Test Description">`,
			expectedProperty: "description",
			expectedContent:  "Test Description",
		},
		{
			name:             "Regular meta tag (name=keywords, not approved)",
			nodeHTML:         `<meta name="keywords" content="Test Keywords">`,
			expectedProperty: "",
			expectedContent:  "Test Keywords",
		},
		{
			name:             "Twitter tag (not approved by default)",
			nodeHTML:         `<meta name="twitter:card" content="summary">`,
			expectedProperty: "",
			expectedContent:  "summary",
		},
		{
			name:             "No content (but approved property)",
			nodeHTML:         `<meta property="og:title">`,
			expectedProperty: "og:title",
			expectedContent:  "",
		},
		{
			name:             "No property/name attribute",
			nodeHTML:         `<meta content="No property">`,
			expectedProperty: "",
			expectedContent:  "No property",
		},
		{
			name:             "Explicitly approved tag with property attribute",
			nodeHTML:         `<meta property="description" content="Approved Description Tag">`,
			expectedProperty: "description", // Approved by exact match in approvedTags
			expectedContent:  "Approved Description Tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullHTML := "<html><head>" + tt.nodeHTML + "</head><body></body></html>"
			doc, err := html.Parse(strings.NewReader(fullHTML))
			if err != nil {
				t.Fatalf("failed to parse HTML: %v", err)
			}

			var node *html.Node
			var findMetaNode func(*html.Node)
			findMetaNode = func(n *html.Node) {
				if node != nil { // Stop searching once found
					return
				}
				if n.Type == html.ElementNode && n.Data == "meta" {
					node = n
					return
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					findMetaNode(c)
				}
			}
			findMetaNode(doc) // Start search from root

			if node == nil {
				// Handle cases where the input might not actually contain a meta tag, though all test cases do.
				// If the test case is *designed* not to have a meta tag, this check should be different.
				// But for these tests, failure to find implies an issue with the test setup or parser.
				t.Fatalf("Could not find meta node in test HTML: %s", tt.nodeHTML)
			}

			// Call extractMetaTagInfo using the test cache instance
			property, content := testCache.extractMetaTagInfo(node)

			if property != tt.expectedProperty {
				t.Errorf("expected property '%s', got '%s'", tt.expectedProperty, property)
			}

			if content != tt.expectedContent {
				t.Errorf("expected content '%s', got '%s'", tt.expectedContent, content)
			}
		})
	}
}

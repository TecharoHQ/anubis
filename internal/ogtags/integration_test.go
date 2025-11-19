package ogtags

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

func TestIntegrationGetOGTags(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch r.URL.Path {
		case "/simple":
			w.Write([]byte(`
				<!DOCTYPE html>
				<html>
				<head>
					<meta property="og:title" content="Simple Page" />
					<meta property="og:type" content="website" />
				</head>
				<body><p>Simple page content</p></body>
				</html>
			`))
		case "/complete":
			w.Write([]byte(`
				<!DOCTYPE html>
				<html>
				<head>
					<meta property="og:title" content="Complete Page" />
					<meta property="og:description" content="A page with many OG tags" />
					<meta property="og:image" content="http://example.com/image.jpg" />
					<meta property="og:url" content="http://example.com/complete" />
					<meta property="og:type" content="article" />
				</head>
				<body><p>Complete page content</p></body>
				</html>
			`))
		case "/no-og":
			w.Write([]byte(`
				<!DOCTYPE html>
				<html>
				<head>
					<title>No OG Tags</title>
				</head>
				<body><p>No OG tags here</p></body>
				</html>
			`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Test with different configurations
	testCases := []struct {
		expectedTags map[string]string
		name         string
		path         string
		query        string
		expectError  bool
	}{
		{
			name:  "Simple page",
			path:  "/simple",
			query: "",
			expectedTags: map[string]string{
				"og:title": "Simple Page",
				"og:type":  "website",
			},
			expectError: false,
		},
		{
			name:  "Complete page",
			path:  "/complete",
			query: "ref=test",
			expectedTags: map[string]string{
				"og:title":       "Complete Page",
				"og:description": "A page with many OG tags",
				"og:image":       "http://example.com/image.jpg",
				"og:url":         "http://example.com/complete",
				"og:type":        "article",
			},
			expectError: false,
		},
		{
			name:         "Page with no OG tags",
			path:         "/no-og",
			query:        "",
			expectedTags: map[string]string{},
			expectError:  false,
		},
		{
			name:         "Nonexistent page",
			path:         "/not-found",
			query:        "",
			expectedTags: nil,
			expectError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create cache instance
			cache := NewOGTagCache(ts.URL, config.OpenGraph{
				Enabled:      true,
				TimeToLive:   time.Minute,
				ConsiderHost: false,
			}, memory.New(t.Context()), TargetOptions{})

			// Create URL for test
			testURL, _ := url.Parse(ts.URL)
			testURL.Path = tc.path
			testURL.RawQuery = tc.query

			// Get OG tags
			// Pass the host from the test URL
			ogTags, err := cache.GetOGTags(t.Context(), testURL, testURL.Host)

			// Check error expectation
			if tc.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify all expected tags are present
			for key, expectedValue := range tc.expectedTags {
				if value, ok := ogTags[key]; !ok || value != expectedValue {
					t.Errorf("expected %s: %s, got: %s", key, expectedValue, value)
				}
			}

			// Verify no extra tags are present
			if len(ogTags) != len(tc.expectedTags) {
				t.Errorf("expected %d tags, got %d", len(tc.expectedTags), len(ogTags))
			}

			// Test cache retrieval
			// Pass the host from the test URL
			cachedOGTags, err := cache.GetOGTags(t.Context(), testURL, testURL.Host)
			if err != nil {
				t.Fatalf("failed to get OG tags from cache: %v", err)
			}

			// Verify cached tags match
			for key, expectedValue := range tc.expectedTags {
				if value, ok := cachedOGTags[key]; !ok || value != expectedValue {
					t.Errorf("cached value - expected %s: %s, got: %s", key, expectedValue, value)
				}
			}
		})
	}
}

package ogtags

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestCheckCache(t *testing.T) {
	cache := NewOGTagCache("http://example.com", true, time.Minute)

	// Set up test data
	urlStr := "http://example.com/page"
	expectedTags := map[string]string{
		"og:title":       "Test Title",
		"og:description": "Test Description",
	}

	// Test cache miss
	tags := cache.checkCache(urlStr)
	if tags != nil {
		t.Errorf("expected nil tags on cache miss, got %v", tags)
	}

	// Manually add to cache
	cache.cache.Set(urlStr, expectedTags, time.Minute)

	// Test cache hit
	tags = cache.checkCache(urlStr)
	if tags == nil {
		t.Fatal("expected non-nil tags on cache hit, got nil")
	}

	for key, expectedValue := range expectedTags {
		if value, ok := tags[key]; !ok || value != expectedValue {
			t.Errorf("expected %s: %s, got: %s", key, expectedValue, value)
		}
	}
}

func TestGetOGTags(t *testing.T) {
	var loadCount int // Counter to track how many times the test route is loaded

	// Create a test server to serve a sample HTML page with OG tags
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loadCount++
		if loadCount > 1 {
			t.Fatalf("Test route loaded more than once, cache failed")
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head>
				<meta property="og:title" content="Test Title" />
				<meta property="og:description" content="Test Description" />
				<meta property="og:image" content="http://example.com/image.jpg" />
			</head>
			<body>
				<p>Hello, world!</p>
			</body>
			</html>
		`))
	}))
	defer ts.Close()

	// Create an instance of OGTagCache with a short TTL for testing
	cache := NewOGTagCache(ts.URL, true, 1*time.Minute)

	// Parse the test server URL
	parsedURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}

	// Test fetching OG tags from the test server
	ogTags, err := cache.GetOGTags(parsedURL)
	if err != nil {
		t.Fatalf("failed to get OG tags: %v", err)
	}

	// Verify the fetched OG tags
	expectedTags := map[string]string{
		"og:title":       "Test Title",
		"og:description": "Test Description",
		"og:image":       "http://example.com/image.jpg",
	}

	for key, expectedValue := range expectedTags {
		if value, ok := ogTags[key]; !ok || value != expectedValue {
			t.Errorf("expected %s: %s, got: %s", key, expectedValue, value)
		}
	}

	// Test fetching OG tags from the cache
	ogTags, err = cache.GetOGTags(parsedURL)
	if err != nil {
		t.Fatalf("failed to get OG tags from cache: %v", err)
	}

	// Test fetching OG tags from the cache (3rd time)
	newOgTags, err := cache.GetOGTags(parsedURL)
	if err != nil {
		t.Fatalf("failed to get OG tags from cache: %v", err)
	}

	// Verify the cached OG tags
	for key, expectedValue := range expectedTags {
		if value, ok := ogTags[key]; !ok || value != expectedValue {
			t.Errorf("expected %s: %s, got: %s", key, expectedValue, value)
		}

		initialValue := ogTags[key]
		cachedValue, ok := newOgTags[key]
		if !ok || initialValue != cachedValue {
			t.Errorf("Cache does not line up: expected %s: %s, got: %s", key, initialValue, cachedValue)
		}

	}
}

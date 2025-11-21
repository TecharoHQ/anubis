package ogtags

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
	"golang.org/x/net/html"
)

// FuzzGetTarget tests getTarget with various inputs
func FuzzGetTarget(f *testing.F) {
	// Seed corpus with interesting test cases
	testCases := []struct {
		target string
		path   string
		query  string
	}{
		{"http://example.com", "/", ""},
		{"http://example.com", "/path", "q=1"},
		{"unix:///tmp/socket", "/api", "key=value"},
		{"https://example.com:8080", "/path/to/resource", "a=1&b=2"},
		{"http://example.com", "/path with spaces", "q=hello world"},
		{"http://example.com", "/path/❤️/emoji", "emoji=🎉"},
		{"http://example.com", "/path/../../../etc/passwd", ""},
		{"http://example.com", "/path%2F%2E%2E%2F", "q=%3Cscript%3E"},
		{"unix:///var/run/app.sock", "/../../etc/passwd", ""},
		{"http://[::1]:8080", "/ipv6", "test=1"},
		{"http://example.com", strings.Repeat("/very/long/path", 100), strings.Repeat("param=value&", 100)},
		{"http://example.com", "/path%20with%20encoded", "q=%20encoded%20"},
		{"http://example.com", "/пример/кириллица", "q=тест"},
		{"http://example.com", "/中文/路径", "查询=值"},
		{"", "/path", "q=1"}, // Empty target
	}

	for _, tc := range testCases {
		f.Add(tc.target, tc.path, tc.query)
	}

	f.Fuzz(func(t *testing.T, target, path, query string) {
		// Skip invalid UTF-8 to focus on realistic inputs
		if !utf8.ValidString(target) || !utf8.ValidString(path) || !utf8.ValidString(query) {
			t.Skip()
		}

		// Create cache - should not panic
		cache := NewOGTagCache(target, config.OpenGraph{}, memory.New(context.Background()), TargetOptions{})

		// Create URL
		u := &url.URL{
			Path:     path,
			RawQuery: query,
		}

		// Call getTarget - should not panic
		result := cache.getTarget(u)

		// Basic validation
		if result == "" {
			t.Errorf("getTarget returned empty string for target=%q, path=%q, query=%q", target, path, query)
		}

		// Verify result is a valid URL (for non-empty targets)
		if target != "" {
			parsedResult, err := url.Parse(result)
			if err != nil {
				t.Errorf("getTarget produced invalid URL %q: %v", result, err)
			} else {
				// For unix sockets, verify the scheme is http
				if strings.HasPrefix(target, "unix:") && parsedResult.Scheme != "http" {
					t.Errorf("Unix socket URL should have http scheme, got %q", parsedResult.Scheme)
				}
			}
		}

		// Ensure no memory corruption by calling multiple times
		for i := 0; i < 3; i++ {
			result2 := cache.getTarget(u)
			if result != result2 {
				t.Errorf("getTarget not deterministic: %q != %q", result, result2)
			}
		}
	})
}

// FuzzExtractOGTags tests extractOGTags with various HTML inputs
func FuzzExtractOGTags(f *testing.F) {
	// Seed corpus with interesting HTML cases
	htmlCases := []string{
		`<html><head><meta property="og:title" content="Test"></head></html>`,
		`<meta property="og:title" content="No HTML tags">`,
		`<html><head>` + strings.Repeat(`<meta property="og:title" content="Many tags">`, 1000) + `</head></html>`,
		`<html><head><meta property="og:title" content="<script>alert('xss')</script>"></head></html>`,
		`<html><head><meta property="og:title" content="Line1&#10;Line2"></head></html>`,
		`<html><head><meta property="og:emoji" content="❤️🎉🎊"></head></html>`,
		`<html><head><meta property="og:title" content="` + strings.Repeat("A", 10000) + `"></head></html>`,
		`<html><head><meta property="og:title" content='Single quotes'></head></html>`,
		`<html><head><meta property=og:title content=no-quotes></head></html>`,
		`<html><head><meta name="keywords" content="test,keywords"></head></html>`,
		`<html><head><meta property="unknown:tag" content="Should be ignored"></head></html>`,
		`<html><head><meta property="` + strings.Repeat("og:", 100) + `title" content="Nested prefixes"></head></html>`,
		`<html>` + strings.Repeat(`<div>`, 1000) + `<meta property="og:title" content="Deep nesting">` + strings.Repeat(`</div>`, 1000) + `</html>`,
		`<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><head><meta property="og:title" content="With doctype"/></head></html>`,
		`<html><head><meta property="" content="Empty property"></head></html>`,
		`<html><head><meta content="Content only"></head></html>`,
		`<html><head><meta property="og:title"></head></html>`, // No content
		``, // Empty HTML
		`<html><head><meta property="og:title" content="Кириллица"></head></html>`,
		`<html><head><meta property="og:title" content="中文内容"></head></html>`,
		`<html><head><!--<meta property="og:title" content="Commented out">--></head></html>`,
		`<html><head><META PROPERTY="OG:TITLE" CONTENT="UPPERCASE"></head></html>`,
	}

	for _, htmlc := range htmlCases {
		f.Add(htmlc)
	}

	f.Fuzz(func(t *testing.T, htmlContent string) {
		// Skip invalid UTF-8
		if !utf8.ValidString(htmlContent) {
			t.Skip()
		}

		// Parse HTML - may fail on invalid input
		doc, err := html.Parse(strings.NewReader(htmlContent))
		if err != nil {
			// This is expected for malformed HTML
			return
		}

		cache := NewOGTagCache("http://example.com", config.OpenGraph{}, memory.New(context.Background()), TargetOptions{})

		// Should not panic
		tags := cache.extractOGTags(doc)

		// Validate results
		for property, content := range tags {
			// Ensure property is approved
			approved := false
			for _, prefix := range cache.approvedPrefixes {
				if strings.HasPrefix(property, prefix) {
					approved = true
					break
				}
			}
			if !approved {
				for _, tag := range cache.approvedTags {
					if property == tag {
						approved = true
						break
					}
				}
			}
			if !approved {
				t.Errorf("Unapproved property %q was extracted", property)
			}

			// Ensure content is valid string
			if !utf8.ValidString(content) {
				t.Errorf("Invalid UTF-8 in content for property %q", property)
			}
		}

		// Test determinism
		tags2 := cache.extractOGTags(doc)
		if len(tags) != len(tags2) {
			t.Errorf("extractOGTags not deterministic: different lengths %d != %d", len(tags), len(tags2))
		}
		for k, v := range tags {
			if tags2[k] != v {
				t.Errorf("extractOGTags not deterministic: %q=%q != %q=%q", k, v, k, tags2[k])
			}
		}
	})
}

// FuzzGetTargetRoundTrip tests that getTarget produces valid URLs that can be parsed back
func FuzzGetTargetRoundTrip(f *testing.F) {
	f.Add("http://example.com", "/path/to/resource", "key=value&foo=bar")
	f.Add("unix:///tmp/socket", "/api/endpoint", "param=test")

	f.Fuzz(func(t *testing.T, target, path, query string) {
		if !utf8.ValidString(target) || !utf8.ValidString(path) || !utf8.ValidString(query) {
			t.Skip()
		}

		cache := NewOGTagCache(target, config.OpenGraph{}, memory.New(context.Background()), TargetOptions{})
		u := &url.URL{Path: path, RawQuery: query}

		result := cache.getTarget(u)
		if result == "" {
			return
		}

		// Parse the result back
		parsed, err := url.Parse(result)
		if err != nil {
			t.Errorf("getTarget produced unparseable URL: %v", err)
			return
		}

		// For non-unix targets, verify path preservation (accounting for encoding)
		if !strings.HasPrefix(target, "unix:") && target != "" {
			// The paths should match after normalization
			expectedPath := u.EscapedPath()
			if parsed.EscapedPath() != expectedPath {
				t.Errorf("Path not preserved: want %q, got %q", expectedPath, parsed.EscapedPath())
			}

			// Query should be preserved exactly
			if parsed.RawQuery != query {
				t.Errorf("Query not preserved: want %q, got %q", query, parsed.RawQuery)
			}
		}
	})
}

// FuzzExtractMetaTagInfo tests the extractMetaTagInfo function directly
func FuzzExtractMetaTagInfo(f *testing.F) {
	// Seed with various attribute combinations
	f.Add("og:title", "Test Title", "property")
	f.Add("keywords", "test,keywords", "name")
	f.Add("og:description", "A description with \"quotes\"", "property")
	f.Add("twitter:card", "summary", "property")
	f.Add("unknown:tag", "Should be filtered", "property")
	f.Add("", "Content without property", "property")
	f.Add("og:title", "", "property") // Property without content

	f.Fuzz(func(t *testing.T, propertyValue, contentValue, propertyKey string) {
		if !utf8.ValidString(propertyValue) || !utf8.ValidString(contentValue) || !utf8.ValidString(propertyKey) {
			t.Skip()
		}

		// Create a meta node
		node := &html.Node{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: propertyKey, Val: propertyValue},
				{Key: "content", Val: contentValue},
			},
		}

		cache := NewOGTagCache("http://example.com", config.OpenGraph{}, memory.New(context.Background()), TargetOptions{})

		// Should not panic
		property, content := cache.extractMetaTagInfo(node)

		// If property is returned, it must be approved
		if property != "" {
			approved := false
			for _, prefix := range cache.approvedPrefixes {
				if strings.HasPrefix(property, prefix) {
					approved = true
					break
				}
			}
			if !approved {
				for _, tag := range cache.approvedTags {
					if property == tag {
						approved = true
						break
					}
				}
			}
			if !approved {
				t.Errorf("extractMetaTagInfo returned unapproved property: %q", property)
			}
		}

		// Content should match input if property is approved
		if property != "" && content != contentValue {
			t.Errorf("Content mismatch: want %q, got %q", contentValue, content)
		}
	})
}

// Benchmark comparison for the fuzzed scenarios
func BenchmarkFuzzedGetTarget(b *testing.B) {
	// Test with various challenging inputs found during fuzzing
	inputs := []struct {
		name   string
		target string
		path   string
		query  string
	}{
		{"Simple", "http://example.com", "/api", "k=v"},
		{"LongPath", "http://example.com", strings.Repeat("/segment", 50), ""},
		{"LongQuery", "http://example.com", "/", strings.Repeat("param=value&", 50)},
		{"Unicode", "http://example.com", "/путь/路径/path", "q=значение"},
		{"Encoded", "http://example.com", "/path%20with%20spaces", "q=%3Cscript%3E"},
		{"Unix", "unix:///tmp/socket.sock", "/api/v1/resource", "id=123&format=json"},
	}

	for _, input := range inputs {
		b.Run(input.name, func(b *testing.B) {
			cache := NewOGTagCache(input.target, config.OpenGraph{}, memory.New(context.Background()), TargetOptions{})
			u := &url.URL{Path: input.path, RawQuery: input.query}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = cache.getTarget(u)
			}
		})
	}
}

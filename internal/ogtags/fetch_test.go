package ogtags

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
	"golang.org/x/net/html"
)

func TestFetchHTMLDocument(t *testing.T) {
	tests := []struct {
		name          string
		htmlContent   string
		contentType   string
		statusCode    int
		contentLength int64
		expectError   bool
	}{
		{
			name: "Valid HTML",
			htmlContent: `<!DOCTYPE html>
				<html>
				<head><title>Test</title></head>
				<body><p>Test content</p></body>
				</html>`,
			contentType: "text/html",
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "Empty HTML",
			htmlContent: "",
			contentType: "text/html",
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "Not found error",
			htmlContent: "",
			contentType: "text/html",
			statusCode:  http.StatusNotFound,
			expectError: true,
		},
		{
			name:        "Unsupported Content-Type",
			htmlContent: "*Insert rick roll here*",
			contentType: "video/mp4",
			statusCode:  http.StatusOK,
			expectError: true,
		},
		{
			name:          "Too large content",
			contentType:   "text/html",
			statusCode:    http.StatusOK,
			expectError:   true,
			contentLength: 5 * 1024 * 1024, // 5MB (over 2MB limit)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				if tt.contentLength > 0 {
					// Simulate content length but avoid sending too much actual data
					w.Header().Set("Content-Length", fmt.Sprintf("%d", tt.contentLength))
					io.CopyN(w, strings.NewReader("X"), tt.contentLength)
				} else {
					w.WriteHeader(tt.statusCode)
					w.Write([]byte(tt.htmlContent))
				}
			}))
			defer ts.Close()

			cache := NewOGTagCache("", config.OpenGraph{
				Enabled:      true,
				TimeToLive:   time.Minute,
				ConsiderHost: false,
			}, memory.New(t.Context()), TargetOptions{})
			doc, err := cache.fetchHTMLDocument(t.Context(), ts.URL, "anything")

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if doc != nil {
					t.Error("expected nil document on error, got non-nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if doc == nil {
					t.Error("expected non-nil document, got nil")
				}
			}
		})
	}
}

func TestFetchHTMLDocumentInvalidURL(t *testing.T) {
	if os.Getenv("DONT_USE_NETWORK") != "" {
		t.Skip("test requires theoretical network egress")
	}

	cache := NewOGTagCache("", config.OpenGraph{
		Enabled:      true,
		TimeToLive:   time.Minute,
		ConsiderHost: false,
	}, memory.New(t.Context()), TargetOptions{})

	doc, err := cache.fetchHTMLDocument(t.Context(), "http://invalid.url.that.doesnt.exist.example", "anything")

	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}

	if doc != nil {
		t.Error("expected nil document for invalid URL, got non-nil")
	}
}

// fetchHTMLDocument allows you to call fetchHTMLDocumentWithCache without a duplicate generateCacheKey call
func (c *OGTagCache) fetchHTMLDocument(ctx context.Context, urlStr string, originalHost string) (*html.Node, error) {
	cacheKey := c.generateCacheKey(urlStr, originalHost)
	return c.fetchHTMLDocumentWithCache(ctx, urlStr, originalHost, cacheKey)
}

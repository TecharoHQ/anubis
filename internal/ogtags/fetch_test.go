package ogtags

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchHTMLDocument(t *testing.T) {
	tests := []struct {
		name        string
		htmlContent string
		statusCode  int
		expectError bool
	}{
		{
			name: "Valid HTML",
			htmlContent: `<!DOCTYPE html>
				<html>
				<head><title>Test</title></head>
				<body><p>Test content</p></body>
				</html>`,
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "Empty HTML",
			htmlContent: "",
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "Not found error",
			htmlContent: "Not Found",
			statusCode:  http.StatusNotFound,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.htmlContent))
			}))
			defer ts.Close()

			cache := NewOGTagCache("", true, time.Minute, false)
			doc, err := cache.fetchHTMLDocument(ts.URL)

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
	cache := NewOGTagCache("", true, time.Minute, false)

	// Test with invalid URL
	doc, err := cache.fetchHTMLDocument("http://invalid.url.that.doesnt.exist.example")

	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}

	if doc != nil {
		t.Error("expected nil document for invalid URL, got non-nil")
	}
}

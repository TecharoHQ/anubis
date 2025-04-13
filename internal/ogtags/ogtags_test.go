package ogtags

import (
	"net/url"
	"testing"
	"time"
)

func TestNewOGTagCache(t *testing.T) {
	tests := []struct {
		name          string
		target        string
		ogPassthrough bool
		ogTimeToLive  time.Duration
	}{
		{
			name:          "Basic initialization",
			target:        "http://example.com",
			ogPassthrough: true,
			ogTimeToLive:  5 * time.Minute,
		},
		{
			name:          "Empty target",
			target:        "",
			ogPassthrough: false,
			ogTimeToLive:  10 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewOGTagCache(tt.target, tt.ogPassthrough, tt.ogTimeToLive)

			if cache == nil {
				t.Fatal("expected non-nil cache, got nil")
			}

			if cache.target != tt.target {
				t.Errorf("expected target %s, got %s", tt.target, cache.target)
			}

			if cache.ogPassthrough != tt.ogPassthrough {
				t.Errorf("expected ogPassthrough %v, got %v", tt.ogPassthrough, cache.ogPassthrough)
			}

			if cache.ogTimeToLive != tt.ogTimeToLive {
				t.Errorf("expected ogTimeToLive %v, got %v", tt.ogTimeToLive, cache.ogTimeToLive)
			}
		})
	}
}

func TestGetTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		path     string
		query    string
		expected string
	}{
		{
			name:     "No path or query",
			target:   "http://example.com",
			path:     "",
			query:    "",
			expected: "http://example.com",
		},
		{
			name:     "With complex path",
			target:   "http://example.com",
			path:     "/pag(#*((#@)ΓΓΓΓe/Γ",
			query:    "id=123",
			expected: "http://example.com/pag(#*((#@)ΓΓΓΓe/Γ",
		},
		{
			name:     "With query and path",
			target:   "http://example.com",
			path:     "/page",
			query:    "id=123",
			expected: "http://example.com/page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewOGTagCache(tt.target, false, time.Minute)

			u := &url.URL{
				Path:     tt.path,
				RawQuery: tt.query,
			}

			result := cache.getTarget(u)

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

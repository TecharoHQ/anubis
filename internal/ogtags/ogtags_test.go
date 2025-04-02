package ogtags

import (
	"net/url"
	"testing"
	"time"
)

func TestNewOGTagCache(t *testing.T) {
	tests := []struct {
		name            string
		target          string
		ogPassthrough   bool
		ogTimeToLive    time.Duration
		ogQueryDistinct bool
	}{
		{
			name:            "Basic initialization",
			target:          "http://example.com",
			ogPassthrough:   true,
			ogTimeToLive:    5 * time.Minute,
			ogQueryDistinct: true,
		},
		{
			name:            "Empty target",
			target:          "",
			ogPassthrough:   false,
			ogTimeToLive:    10 * time.Minute,
			ogQueryDistinct: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewOGTagCache(tt.target, tt.ogPassthrough, tt.ogTimeToLive, tt.ogQueryDistinct)

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

			if cache.ogQueryDistinct != tt.ogQueryDistinct {
				t.Errorf("expected ogQueryDistinct %v, got %v", tt.ogQueryDistinct, cache.ogQueryDistinct)
			}
		})
	}
}

func TestGetTarget(t *testing.T) {
	tests := []struct {
		name            string
		target          string
		path            string
		query           string
		ogQueryDistinct bool
		expected        string
	}{
		{
			name:            "No query, query distinct false",
			target:          "http://example.com",
			path:            "/page",
			query:           "",
			ogQueryDistinct: false,
			expected:        "http://example.com/page",
		},
		{
			name:            "With query, query distinct true",
			target:          "http://example.com",
			path:            "/page",
			query:           "id=123",
			ogQueryDistinct: true,
			expected:        "http://example.com/page?id=123",
		},
		{
			name:            "With query, query distinct false",
			target:          "http://example.com",
			path:            "/page",
			query:           "id=123",
			ogQueryDistinct: false,
			expected:        "http://example.com/page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewOGTagCache(tt.target, false, time.Minute, tt.ogQueryDistinct)

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

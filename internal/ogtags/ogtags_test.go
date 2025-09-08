package ogtags

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
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
			cache := NewOGTagCache(tt.target, config.OpenGraph{
				Enabled:      tt.ogPassthrough,
				TimeToLive:   tt.ogTimeToLive,
				ConsiderHost: false,
			}, memory.New(t.Context()))

			if cache == nil {
				t.Fatal("expected non-nil cache, got nil")
			}

			// Check the parsed targetURL, handling the default case for empty target
			expectedURLStr := tt.target
			if tt.target == "" {
				// Default behavior when target is empty is now http://localhost
				expectedURLStr = "http://localhost"
			} else if !strings.Contains(tt.target, "://") && !strings.HasPrefix(tt.target, "unix:") {
				// Handle case where target is just host or host:port (and not unix)
				expectedURLStr = "http://" + tt.target
			}
			if cache.targetURL.String() != expectedURLStr {
				t.Errorf("expected targetURL %s, got %s", expectedURLStr, cache.targetURL.String())
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

// TestNewOGTagCache_UnixSocket specifically tests unix socket initialization
func TestNewOGTagCache_UnixSocket(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test.sock")
	target := "unix://" + socketPath

	cache := NewOGTagCache(target, config.OpenGraph{
		Enabled:      true,
		TimeToLive:   5 * time.Minute,
		ConsiderHost: false,
	}, memory.New(t.Context()))

	if cache == nil {
		t.Fatal("expected non-nil cache, got nil")
	}

	if cache.targetURL.Scheme != "unix" {
		t.Errorf("expected targetURL scheme 'unix', got '%s'", cache.targetURL.Scheme)
	}
	if cache.targetURL.Path != socketPath {
		t.Errorf("expected targetURL path '%s', got '%s'", socketPath, cache.targetURL.Path)
	}

	// Check if the client transport is configured for Unix sockets
	transport, ok := cache.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected client transport to be *http.Transport, got %T", cache.client.Transport)
	}
	if transport.DialContext == nil {
		t.Fatal("expected client transport DialContext to be non-nil for unix socket")
	}

	// Attempt a dummy dial to see if it uses the correct path (optional, more involved check)
	dummyConn, err := transport.DialContext(context.Background(), "", "")
	if err == nil {
		dummyConn.Close()
		t.Log("DialContext seems functional, but couldn't verify path without a listener")
	} else if !strings.Contains(err.Error(), "connect: connection refused") && !strings.Contains(err.Error(), "connect: no such file or directory") {
		// We expect connection refused or not found if nothing is listening
		t.Errorf("DialContext failed with unexpected error: %v", err)
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
			name:   "With complex path",
			target: "http://example.com",
			path:   "/pag(#*((#@)ΓΓΓΓe/Γ",
			query:  "id=123",
			// Expect URL encoding and query parameter
			expected: "http://example.com/pag%28%23%2A%28%28%23@%29%CE%93%CE%93%CE%93%CE%93e/%CE%93?id=123",
		},
		{
			name:     "With query and path",
			target:   "http://example.com",
			path:     "/page",
			query:    "id=123",
			expected: "http://example.com/page?id=123",
		},
		{
			name:     "Unix socket target",
			target:   "unix:/tmp/anubis.sock",
			path:     "/some/path",
			query:    "key=value&flag=true",
			expected: "http://unix/some/path?key=value&flag=true", // Scheme becomes http, host is 'unix'
		},
		{
			name:     "Unix socket target with ///",
			target:   "unix:///var/run/anubis.sock",
			path:     "/",
			query:    "",
			expected: "http://unix/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewOGTagCache(tt.target, config.OpenGraph{
				Enabled:      true,
				TimeToLive:   time.Minute,
				ConsiderHost: false,
			}, memory.New(t.Context()))

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

// TestIntegrationGetOGTags_UnixSocket tests fetching OG tags via a Unix socket.
func TestIntegrationGetOGTags_UnixSocket(t *testing.T) {
	tempDir := t.TempDir()

	// XXX(Xe): if this is named longer, macOS fails with `bind: invalid argument`
	// because the unix socket path is too long. I love computers.
	socketPath := filepath.Join(tempDir, "t")

	// Ensure the socket does not exist initially
	_ = os.Remove(socketPath)

	// Create a simple HTTP server listening on the Unix socket
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to listen on unix socket %s: %v", socketPath, err)
	}
	defer func(listener net.Listener, socketPath string) {
		if listener != nil {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Logf("Error closing listener: %v", err)
			}
		}

		if _, err := os.Stat(socketPath); err == nil {
			if err := os.Remove(socketPath); err != nil {
				t.Logf("Error removing socket file %s: %v", socketPath, err)
			}
		}
	}(listener, socketPath)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, `<!DOCTYPE html><html><head><meta property="og:title" content="Unix Socket Test" /></head><body>Test</body></html>`)
		}),
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Unix socket server error: %v", err)
		}
	}()
	defer func(server *http.Server, ctx context.Context) {
		err := server.Shutdown(ctx)
		if err != nil {
			t.Logf("Error shutting down server: %v", err)
		}
	}(server, context.Background()) // Ensure server is shut down

	// Wait a moment for the server to start
	time.Sleep(100 * time.Millisecond)

	// Create cache instance pointing to the Unix socket
	targetURL := "unix://" + socketPath
	cache := NewOGTagCache(targetURL, config.OpenGraph{
		Enabled:      true,
		TimeToLive:   time.Minute,
		ConsiderHost: false,
	}, memory.New(t.Context()))

	// Create a dummy URL for the request (path and query matter)
	testReqURL, _ := url.Parse("/some/page?query=1")

	// Get OG tags
	// Pass an empty string for host, as it's irrelevant for unix sockets
	ogTags, err := cache.GetOGTags(t.Context(), testReqURL, "")

	if err != nil {
		t.Fatalf("GetOGTags failed for unix socket: %v", err)
	}

	expectedTags := map[string]string{
		"og:title": "Unix Socket Test",
	}

	if !reflect.DeepEqual(ogTags, expectedTags) {
		t.Errorf("Expected OG tags %v, got %v", expectedTags, ogTags)
	}

	// Test cache retrieval (should hit cache)
	// Pass an empty string for host
	cachedTags, err := cache.GetOGTags(t.Context(), testReqURL, "")
	if err != nil {
		t.Fatalf("GetOGTags (cache hit) failed for unix socket: %v", err)
	}
	if !reflect.DeepEqual(cachedTags, expectedTags) {
		t.Errorf("Expected cached OG tags %v, got %v", expectedTags, cachedTags)
	}
}

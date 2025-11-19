package ogtags

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/config"
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
			}, memory.New(t.Context()), TargetOptions{})

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
	}, memory.New(t.Context()), TargetOptions{})

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
			}, memory.New(t.Context()), TargetOptions{})

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
	}, memory.New(t.Context()), TargetOptions{})

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

func TestGetOGTagsWithTargetHostOverride(t *testing.T) {
	originalHost := "example.test"
	overrideHost := "backend.internal"
	seenHosts := make(chan string, 10)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHosts <- r.Host
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<!DOCTYPE html><html><head><meta property="og:title" content="HostOverride" /></head><body>ok</body></html>`)
	}))
	defer ts.Close()

	targetURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}

	conf := config.OpenGraph{
		Enabled:      true,
		TimeToLive:   time.Minute,
		ConsiderHost: false,
	}

	t.Run("default host uses original", func(t *testing.T) {
		cache := NewOGTagCache(ts.URL, conf, memory.New(t.Context()), TargetOptions{})
		if _, err := cache.GetOGTags(t.Context(), targetURL, originalHost); err != nil {
			t.Fatalf("GetOGTags failed: %v", err)
		}
		select {
		case host := <-seenHosts:
			if host != originalHost {
				t.Fatalf("expected host %q, got %q", originalHost, host)
			}
		case <-time.After(time.Second):
			t.Fatal("server did not receive request")
		}
	})

	t.Run("override host respected", func(t *testing.T) {
		cache := NewOGTagCache(ts.URL, conf, memory.New(t.Context()), TargetOptions{
			Host: overrideHost,
		})
		if _, err := cache.GetOGTags(t.Context(), targetURL, originalHost); err != nil {
			t.Fatalf("GetOGTags failed: %v", err)
		}
		select {
		case host := <-seenHosts:
			if host != overrideHost {
				t.Fatalf("expected host %q, got %q", overrideHost, host)
			}
		case <-time.After(time.Second):
			t.Fatal("server did not receive request")
		}
	})
}

func TestGetOGTagsWithInsecureSkipVerify(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<!DOCTYPE html><html><head><meta property="og:title" content="Self-Signed" /></head><body>hello</body></html>`)
	})
	ts := httptest.NewTLSServer(handler)
	defer ts.Close()

	parsedURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}

	conf := config.OpenGraph{
		Enabled:      true,
		TimeToLive:   time.Minute,
		ConsiderHost: false,
	}

	// Without skip verify we should get a TLS error
	cacheStrict := NewOGTagCache(ts.URL, conf, memory.New(t.Context()), TargetOptions{})
	if _, err := cacheStrict.GetOGTags(t.Context(), parsedURL, parsedURL.Host); err == nil {
		t.Fatal("expected TLS verification error without InsecureSkipVerify")
	}

	cacheSkip := NewOGTagCache(ts.URL, conf, memory.New(t.Context()), TargetOptions{
		InsecureSkipVerify: true,
	})

	tags, err := cacheSkip.GetOGTags(t.Context(), parsedURL, parsedURL.Host)
	if err != nil {
		t.Fatalf("expected successful fetch with InsecureSkipVerify, got: %v", err)
	}
	if tags["og:title"] != "Self-Signed" {
		t.Fatalf("expected og:title to be %q, got %q", "Self-Signed", tags["og:title"])
	}
}

func TestGetOGTagsWithTargetSNI(t *testing.T) {
	originalHost := "hecate.test"
	conf := config.OpenGraph{
		Enabled:      true,
		TimeToLive:   time.Minute,
		ConsiderHost: false,
	}

	t.Run("explicit SNI override", func(t *testing.T) {
		expectedSNI := "backend.internal"
		ts, recorder := newSNIServer(t, `<!DOCTYPE html><html><head><meta property="og:title" content="SNI Works" /></head><body>ok</body></html>`)
		defer ts.Close()

		targetURL, err := url.Parse(ts.URL)
		if err != nil {
			t.Fatalf("failed to parse server URL: %v", err)
		}

		cacheExplicit := NewOGTagCache(ts.URL, conf, memory.New(t.Context()), TargetOptions{
			SNI:                expectedSNI,
			InsecureSkipVerify: true,
		})
		if _, err := cacheExplicit.GetOGTags(t.Context(), targetURL, originalHost); err != nil {
			t.Fatalf("expected successful fetch with explicit SNI, got: %v", err)
		}
		if got := recorder.last(); got != expectedSNI {
			t.Fatalf("expected server to see SNI %q, got %q", expectedSNI, got)
		}
	})

	t.Run("auto SNI uses original host", func(t *testing.T) {
		ts, recorder := newSNIServer(t, `<!DOCTYPE html><html><head><meta property="og:title" content="SNI Auto" /></head><body>ok</body></html>`)
		defer ts.Close()

		targetURL, err := url.Parse(ts.URL)
		if err != nil {
			t.Fatalf("failed to parse server URL: %v", err)
		}

		cacheAuto := NewOGTagCache(ts.URL, conf, memory.New(t.Context()), TargetOptions{
			SNI:                "auto",
			InsecureSkipVerify: true,
		})
		if _, err := cacheAuto.GetOGTags(t.Context(), targetURL, originalHost); err != nil {
			t.Fatalf("expected successful fetch with auto SNI, got: %v", err)
		}
		if got := recorder.last(); got != originalHost {
			t.Fatalf("expected server to see SNI %q with auto, got %q", originalHost, got)
		}
	})

	t.Run("default SNI uses backend host", func(t *testing.T) {
		ts, recorder := newSNIServer(t, `<!DOCTYPE html><html><head><meta property="og:title" content="SNI Default" /></head><body>ok</body></html>`)
		defer ts.Close()

		targetURL, err := url.Parse(ts.URL)
		if err != nil {
			t.Fatalf("failed to parse server URL: %v", err)
		}

		cacheDefault := NewOGTagCache(ts.URL, conf, memory.New(t.Context()), TargetOptions{
			InsecureSkipVerify: true,
		})
		if _, err := cacheDefault.GetOGTags(t.Context(), targetURL, originalHost); err != nil {
			t.Fatalf("expected successful fetch without explicit SNI, got: %v", err)
		}
		wantSNI := ""
		if net.ParseIP(targetURL.Hostname()) == nil {
			wantSNI = targetURL.Hostname()
		}
		if got := recorder.last(); got != wantSNI {
			t.Fatalf("expected default SNI %q, got %q", wantSNI, got)
		}
	})
}

func newSNIServer(t *testing.T, body string) (*httptest.Server, *sniRecorder) {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body)
	})

	recorder := &sniRecorder{}
	ts := httptest.NewUnstartedServer(handler)
	cert := mustCertificateForHost(t, "sni.test")
	ts.TLS = &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			recorder.record(hello.ServerName)
			return &cert, nil
		},
	}
	ts.StartTLS()
	return ts, recorder
}

func mustCertificateForHost(t *testing.T, host string) tls.Certificate {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		DNSNames:              []string{host},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}
}

type sniRecorder struct {
	mu    sync.Mutex
	names []string
}

func (r *sniRecorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
}

func (r *sniRecorder) last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.names) == 0 {
		return ""
	}
	return r.names[len(r.names)-1]
}

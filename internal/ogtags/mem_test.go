package ogtags

import (
	"net/url"
	"runtime"
	"strings"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
	"golang.org/x/net/html"
)

func BenchmarkGetTarget(b *testing.B) {
	tests := []struct {
		name   string
		target string
		paths  []string
	}{
		{
			name:   "HTTP",
			target: "http://example.com",
			paths:  []string{"/", "/path", "/path/to/resource", "/path?query=1&foo=bar"},
		},
		{
			name:   "Unix",
			target: "unix:///var/run/app.sock",
			paths:  []string{"/", "/api/endpoint", "/api/endpoint?param=value"},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			cache := NewOGTagCache(tt.target, config.OpenGraph{}, memory.New(b.Context()), TargetOptions{})
			urls := make([]*url.URL, len(tt.paths))
			for i, path := range tt.paths {
				u, _ := url.Parse(path)
				urls[i] = u
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = cache.getTarget(urls[i%len(urls)])
			}
		})
	}
}

func BenchmarkExtractOGTags(b *testing.B) {
	htmlSamples := []string{
		`<html><head>
			<meta property="og:title" content="Test Title">
			<meta property="og:description" content="Test Description">
			<meta name="keywords" content="test,keywords">
		</head><body></body></html>`,
		`<html><head>
			<meta property="og:title" content="Page Title">
			<meta property="og:type" content="website">
			<meta property="og:url" content="https://example.com">
			<meta property="og:image" content="https://example.com/image.jpg">
			<meta property="twitter:card" content="summary_large_image">
			<meta property="twitter:title" content="Twitter Title">
			<meta name="description" content="Page description">
			<meta name="author" content="John Doe">
		</head><body><div><p>Content</p></div></body></html>`,
	}

	cache := NewOGTagCache("http://example.com", config.OpenGraph{}, memory.New(b.Context()), TargetOptions{})
	docs := make([]*html.Node, len(htmlSamples))

	for i, sample := range htmlSamples {
		doc, _ := html.Parse(strings.NewReader(sample))
		docs[i] = doc
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = cache.extractOGTags(docs[i%len(docs)])
	}
}

// Memory usage test
func TestMemoryUsage(t *testing.T) {
	cache := NewOGTagCache("http://example.com", config.OpenGraph{}, memory.New(t.Context()), TargetOptions{})

	// Force GC and wait for it to complete
	runtime.GC()

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Run getTarget many times
	u, _ := url.Parse("/path/to/resource?query=1&foo=bar&baz=qux")
	for i := 0; i < 10000; i++ {
		_ = cache.getTarget(u)
	}

	// Force GC after operations
	runtime.GC()

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	allocatedBytes := int64(m2.TotalAlloc) - int64(m1.TotalAlloc)
	allocatedKB := float64(allocatedBytes) / 1024.0
	allocatedPerOp := float64(allocatedBytes) / 10000.0

	t.Logf("Memory allocated for 10k getTarget calls:")
	t.Logf("  Total: %.2f KB (%.2f MB)", allocatedKB, allocatedKB/1024.0)
	t.Logf("  Per operation: %.2f bytes", allocatedPerOp)

	// Test extractOGTags memory usage
	htmlDoc := `<html><head>
		<meta property="og:title" content="Test Title">
		<meta property="og:description" content="Test Description">
		<meta property="og:image" content="https://example.com/image.jpg">
		<meta property="twitter:card" content="summary">
		<meta name="keywords" content="test,keywords,example">
		<meta name="author" content="Test Author">
		<meta property="unknown:tag" content="Should be ignored">
	</head><body></body></html>`

	doc, _ := html.Parse(strings.NewReader(htmlDoc))

	runtime.GC()
	runtime.ReadMemStats(&m1)

	for i := 0; i < 1000; i++ {
		_ = cache.extractOGTags(doc)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	allocatedBytes = int64(m2.TotalAlloc) - int64(m1.TotalAlloc)
	allocatedKB = float64(allocatedBytes) / 1024.0
	allocatedPerOp = float64(allocatedBytes) / 1000.0

	t.Logf("Memory allocated for 1k extractOGTags calls:")
	t.Logf("  Total: %.2f KB (%.2f MB)", allocatedKB, allocatedKB/1024.0)
	t.Logf("  Per operation: %.2f bytes", allocatedPerOp)

	// Sanity checks
	if allocatedPerOp > 10000 {
		t.Errorf("extractOGTags allocating too much memory per operation: %.2f bytes", allocatedPerOp)
	}
}

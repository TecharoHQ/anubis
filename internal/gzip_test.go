package internal

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// gzipTestPayload returns a deterministic, compressible body so the gzip
// middleware tests and benchmark are comparable across runs.
func gzipTestPayload() []byte {
	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i)
	}
	return payload
}

func TestGzipMiddleware(t *testing.T) {
	payload := gzipTestPayload()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	})

	t.Run("compresses when client accepts gzip", func(t *testing.T) {
		h := GzipMiddleware(gzip.BestSpeed, inner)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("Content-Encoding = %q, want %q", got, "gzip")
		}

		gz, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatalf("response body is not valid gzip: %v", err)
		}
		got, err := io.ReadAll(gz)
		if err != nil {
			t.Fatalf("can't read gzip body: %v", err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("decompressed body does not match original (got %d bytes, want %d)", len(got), len(payload))
		}
	})

	t.Run("passes through when client does not accept gzip", func(t *testing.T) {
		h := GzipMiddleware(gzip.BestSpeed, inner)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("Content-Encoding = %q, want empty", got)
		}
		if !bytes.Equal(rec.Body.Bytes(), payload) {
			t.Fatal("body was modified despite client not accepting gzip")
		}
	})
}

// TestGzipMiddlewareInvalidLevel covers the construction-time range check:
// an out-of-range compression level must panic up front rather than mid-request.
func TestGzipMiddlewareInvalidLevel(t *testing.T) {
	for _, tt := range []struct {
		name  string
		level int
	}{
		{"below-huffman-only", gzip.HuffmanOnly - 1},
		{"above-best-compression", gzip.BestCompression + 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("GzipMiddleware(%d, ...) did not panic on invalid level", tt.level)
				}
			}()
			GzipMiddleware(tt.level, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		})
	}
}

// GzipMiddleware is rebuilt on every request at its production
// call site (RenderIndex wraps the request-specific challenge page), so the
// *gzip.Writer pool must outlive any single middleware instance. If the pool is
// ever moved back inside the closure, each request reallocates a ~1.18 MiB
// deflate writer and the per-request allocation count jumps from the mid-teens
// into the high thirties. Assert it stays low.
func TestGzipMiddlewarePoolSurvivesReconstruction(t *testing.T) {
	payload := gzipTestPayload()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	serve := func() {
		// Construct the middleware per call, exactly like RenderIndex does.
		h := GzipMiddleware(gzip.BestSpeed, inner)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		io.Copy(io.Discard, rec.Body)
	}

	const maxAllocs = 25 // healthy steady state is ~14; a defeated pool is ~37
	got := testing.AllocsPerRun(50, serve)
	t.Logf("allocations per (construct + serve): %.0f", got)
	if got > maxAllocs {
		t.Fatalf("GzipMiddleware allocates %.0f times per request; want <= %d. "+
			"The *gzip.Writer pool is likely no longer shared across middleware "+
			"reconstructions (see gzipWriterPools).", got, maxAllocs)
	}
}

// BenchmarkGzipMiddleware exercises the production pattern where the middleware
// is constructed inside the per-request handler. Constructing it once up front
// would hide whether the writer pool actually survives reconstruction, which is
// exactly the regression this benchmark exists to surface.
func BenchmarkGzipMiddleware(b *testing.B) {
	payload := gzipTestPayload()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	})

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		for pb.Next() {
			h := GzipMiddleware(gzip.BestSpeed, inner)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			io.Copy(io.Discard, rec.Body)
		}
	})
}

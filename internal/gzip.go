package internal

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipWriterPools holds one *sync.Pool of *gzip.Writer per compression level.
//
// The pools are process-global on purpose. GzipMiddleware is constructed
// per-request at its RenderIndex call site (the wrapped handler embeds the
// request-specific challenge page), so a closure-local pool would be created
// empty, used once, and garbage collected on every challenge render; never
// actually reusing a writer. Keeping the pools here lets the ~1.18 MiB deflate
// buffers inside each *gzip.Writer survive across requests regardless of how
// often the middleware is rebuilt.
var gzipWriterPools sync.Map // map[int]*sync.Pool

func gzipWriterPool(level int) *sync.Pool {
	if p, ok := gzipWriterPools.Load(level); ok {
		return p.(*sync.Pool)
	}
	p := &sync.Pool{
		New: func() any {
			gz, _ := gzip.NewWriterLevel(io.Discard, level)
			return gz
		},
	}
	actual, _ := gzipWriterPools.LoadOrStore(level, p)
	return actual.(*sync.Pool)
}

func GzipMiddleware(level int, next http.Handler) http.Handler {
	// Validate the level with the same range check gzip.NewWriterLevel uses,
	// but without allocating a throwaway ~1.18 MiB writer on every (per-request)
	// middleware construction. gzip only rejects out-of-range levels.
	if level < gzip.HuffmanOnly || level > gzip.BestCompression {
		panic(fmt.Sprintf("gzip: invalid compression level: %d", level))
	}

	pool := gzipWriterPool(level)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz := pool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer func() {
			gz.Close()
			gz.Reset(io.Discard)
			pool.Put(gz)
		}()

		grw := gzipResponseWriter{ResponseWriter: w, sink: gz}
		next.ServeHTTP(grw, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	sink *gzip.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.sink.Write(b)
}

package internal

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

func GzipMiddleware(level int, next http.Handler) http.Handler {
	// Validate the level once at setup; gzip.NewWriterLevel only fails for
	// invalid levels and we'd rather panic now than mid-request.
	if _, err := gzip.NewWriterLevel(io.Discard, level); err != nil {
		panic(err)
	}

	// Per-middleware pool of *gzip.Writer. Each entry carries ~40 KiB of
	// deflate buffers; reusing them avoids that allocation on every request.
	pool := sync.Pool{
		New: func() any {
			gz, _ := gzip.NewWriterLevel(io.Discard, level)
			return gz
		},
	}

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

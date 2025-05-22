package internal

import (
	"compress/gzip"
	"net/http"
	"strings"
)

func GzipMiddleware(level int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz, err := gzip.NewWriterLevel(w, level)
		if err != nil {
			panic(err)
		}
		defer gz.Close()

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

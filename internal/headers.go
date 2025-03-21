package internal

import (
	"log/slog"
	"net/http"

	"github.com/TecharoHQ/anubis"
)

// UnchangingCache sets the Cache-Control header to cache a response for 1 year if
// and only if the application is compiled in "release" mode by Docker.
func UnchangingCache(next http.Handler) http.Handler {
	if anubis.Version == "devel" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		next.ServeHTTP(w, r)
	})
}

// DefaultXRealIP sets the X-Real-Ip header to the given value if and only if
// it is not an empty string.
func DefaultXRealIP(defaultIP string, next http.Handler) http.Handler {
	if defaultIP == "" {
		slog.Debug("skipping middleware, defaultIP is empty")
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Real-Ip", defaultIP)
		next.ServeHTTP(w, r)
	})
}

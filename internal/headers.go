package internal

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/TecharoHQ/anubis"
	"github.com/sebest/xff"
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

// RemoteXRealIP sets the X-Real-Ip header to the request's real IP if
// the setting is enabled by the user.
func RemoteXRealIP(useRemoteAddress *bool, next http.Handler) http.Handler {
	if useRemoteAddress == nil || *useRemoteAddress == false {
		slog.Debug("skipping middleware, useRemoteAddress is empty")
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Real-Ip", strings.Split(r.RemoteAddr, ":")[0])
		next.ServeHTTP(w, r)
	})
}

// XForwardedForToXRealIP sets the X-Real-Ip header based on the contents
// of the X-Forwarded-For header.
func XForwardedForToXRealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if xffHeader := r.Header.Get("X-Forwarded-For"); r.Header.Get("X-Real-Ip") == "" && xffHeader != "" {
			ip := xff.Parse(xffHeader)
			slog.Debug("setting x-real-ip", "val", ip)
			r.Header.Set("X-Real-Ip", ip)
		}

		next.ServeHTTP(w, r)
	})
}

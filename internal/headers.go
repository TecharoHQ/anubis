package internal

import (
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/TecharoHQ/anubis"
	"github.com/sebest/xff"
)

// UnchangingCache sets the Cache-Control header to cache a response for 1 year if
// and only if the application is compiled in "release" mode by Docker.
func UnchangingCache(next http.Handler) http.Handler {
	//goland:noinspection GoBoolExpressions
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
func RemoteXRealIP(useRemoteAddress bool, bindNetwork string, next http.Handler) http.Handler {
	if !useRemoteAddress {
		slog.Debug("skipping middleware, useRemoteAddress is empty")
		return next
	}

	if bindNetwork == "unix" {
		// For local sockets there is no real remote address but the localhost
		// address should be sensible.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Real-Ip", "127.0.0.1")
			next.ServeHTTP(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			panic(err) // this should never happen
		}
		r.Header.Set("X-Real-Ip", host)
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

// XForwardedForUpdate sets or updates the X-Forwarded-For header, adding
// the known remote address to an existing chain if present
func XForwardedForUpdate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer next.ServeHTTP(w, r)

		remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)

		if parsedRemoteIP := net.ParseIP(remoteIP); parsedRemoteIP != nil && parsedRemoteIP.IsLoopback() {
			// anubis is likely deployed behind a local reverse proxy
			// pass header as-is to not break existing applications
			return
		}

		if err != nil {
			slog.Warn("The default format of request.RemoteAddr should be IP:Port", "remoteAddr", r.RemoteAddr)
			return
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			forwardedList := strings.Split(",", xff)
			forwardedList = append(forwardedList, remoteIP)
			// this behavior is equivalent to
			// ingress-nginx "compute-full-forwarded-for"
			// https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/#compute-full-forwarded-for
			//
			// this would be the correct place to strip and/or flatten this list
			//
			// strip - iterate backwards and eliminate configured trusted IPs
			// flatten - only return the last element to avoid spoofing confusion
			//
			// many applications handle this in different ways, but
			// generally they'd be expected to do these two things on
			// their own end to find the first non-spoofed IP
			r.Header.Set("X-Forwarded-For", strings.Join(forwardedList, ","))
		} else {
			r.Header.Set("X-Forwarded-For", remoteIP)
		}
	})
}

// NoStoreCache sets the Cache-Control header to no-store for the response.
func NoStoreCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// Do not allow browsing directory listings in paths that end with /
func NoBrowsing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

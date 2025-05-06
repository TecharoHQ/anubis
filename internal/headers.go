package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/TecharoHQ/anubis"
	"github.com/sebest/xff"
)

// TODO: move into config
type XFFComputePreferences struct {
	StripPrivate  bool
	StripLoopback bool
	StripCGNAT    bool
	StripLLU      bool
	Flatten       bool
}

var CGNat = netip.MustParsePrefix("100.64.0.0/10")

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

		pref := XFFComputePreferences{
			StripPrivate:  true,
			StripLoopback: true,
			StripCGNAT:    true,
			Flatten:       true,
			StripLLU:      true,
		}

		remoteAddr := r.RemoteAddr
		origXFFHeader := r.Header.Get("X-Forwarded-For")

		if remoteAddr == "@" {
			// remote is a unix socket
			// do not touch chain
			return
		}

		xffHeaderString, err := computeXFFHeader(remoteAddr, origXFFHeader, pref)
		if err != nil {
			slog.Debug("computing X-Forwarded-For header failed", "err", err)
			return
		}

		if len(xffHeaderString) == 0 {
			r.Header.Del("X-Forwarded-For")
		} else {
			r.Header.Set("X-Forwarded-For", xffHeaderString)
		}
	})
}

var (
	ErrCantSplitHostParse = errors.New("internal: unable to net.SplitHostParse")
	ErrCantParseRemoteIP  = errors.New("internal: unable to parse remote IP")
)

func computeXFFHeader(remoteAddr string, origXFFHeader string, pref XFFComputePreferences) (string, error) {
	remoteIP, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCantSplitHostParse, err)
	}
	parsedRemoteIP, err := netip.ParseAddr(remoteIP)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCantParseRemoteIP, err)
	}

	origForwardedList := make([]string, 0, 4)
	if origXFFHeader != "" {
		origForwardedList = strings.Split(origXFFHeader, ",")
		for i := range origForwardedList {
			origForwardedList[i] = strings.TrimSpace(origForwardedList[i])
		}
	}
	origForwardedList = append(origForwardedList, parsedRemoteIP.String())
	forwardedList := make([]string, 0, len(origForwardedList))
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
	for i := len(origForwardedList) - 1; i >= 0; i-- {
		segmentIP, err := netip.ParseAddr(origForwardedList[i])
		if err != nil {
			// can't assess this element, so the remainder of the chain
			// can't be trusted. not a fatal error, since anyone can
			// spoof an XFF header
			slog.Debug("failed to parse XFF segment", "err", err)
			break
		}
		if pref.StripPrivate && segmentIP.IsPrivate() {
			continue
		}
		if pref.StripLoopback && segmentIP.IsLoopback() {
			continue
		}
		if pref.StripLLU && segmentIP.IsLinkLocalUnicast() {
			continue
		}
		if pref.StripCGNAT && CGNat.Contains(segmentIP) {
			continue
		}
		forwardedList = append([]string{segmentIP.String()}, forwardedList...)
	}
	var xffHeaderString string
	if len(forwardedList) == 0 {
		xffHeaderString = ""
		return xffHeaderString, nil
	}
	if pref.Flatten {
		xffHeaderString = forwardedList[len(forwardedList)-1]
	} else {
		xffHeaderString = strings.Join(forwardedList, ",")
	}
	return xffHeaderString, nil
}

// NoStoreCache sets the Cache-Control header to no-store for the response.
func NoStoreCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// NoBrowsing prevents directory browsing by returning a 404 for any request that ends with a "/".
func NoBrowsing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

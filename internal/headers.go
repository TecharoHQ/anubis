package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"github.com/TecharoHQ/anubis"
	"github.com/sebest/xff"
)

type realIPKey struct{}

func RealIP(r *http.Request) (netip.Addr, bool) {
	result, ok := r.Context().Value(realIPKey{}).(netip.Addr)
	return result, ok
}

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

// CustomXRealIPHeader sets the X-Real-IP header to the value of a
// different header.
// Used in environments where the upstream proxy sets the request's
// origin IP in a custom header.
func CustomRealIPHeader(customRealIPHeaderValue string, next http.Handler) http.Handler {
	if customRealIPHeaderValue == "" {
		slog.Debug("skipping middleware, customRealIPHeaderValue is empty")
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Real-IP", r.Header.Get(customRealIPHeaderValue))
		next.ServeHTTP(w, r)
	})
}

// RemoteXRealIP sets the X-Real-IP header to the request's real IP if
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
			r.Header.Set("X-Real-IP", "127.0.0.1")
			next.ServeHTTP(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			panic(err) // this should never happen
		}
		r.Header.Set("X-Real-IP", host)
		if addr, err := netip.ParseAddr(host); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), realIPKey{}, addr))
		}
		next.ServeHTTP(w, r)
	})
}

// XForwardedForToXRealIP sets the X-Real-IP header based on the contents
// of the X-Forwarded-For header.
func XForwardedForToXRealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if xffHeader := r.Header.Get("X-Forwarded-For"); r.Header.Get("X-Real-IP") == "" && xffHeader != "" {
			ip := xff.Parse(xffHeader)
			slog.DebugContext(r.Context(), "setting X-Real-IP from X-Forwarded-For", "to", ip, "x-forwarded-for", xffHeader)
			r.Header.Set("X-Real-IP", ip)
			if addr, err := netip.ParseAddr(ip); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), realIPKey{}, addr))
			}
		}

		next.ServeHTTP(w, r)
	})
}

// XForwardedForUpdate sets or updates the X-Forwarded-For header, adding
// the known remote address to an existing chain if present
func XForwardedForUpdate(stripPrivate bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer next.ServeHTTP(w, r)

		pref := XFFComputePreferences{
			StripPrivate:  stripPrivate,
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
			slog.DebugContext(r.Context(), "computing X-Forwarded-For header failed", "err", err)
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

	var origForwardedList []string
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
		segmentIP, err := netip.ParseAddr(strings.TrimSpace(origForwardedList[i]))
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
		// Build the kept chain in reverse (cheap appends into the
		// pre-sized slice) and flip it once below, instead of prepending
		// a freshly allocated slice on every iteration.
		forwardedList = append(forwardedList, segmentIP.String())
	}
	slices.Reverse(forwardedList)
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

var ErrInvalidTrustedProxy = errors.New("internal: invalid trusted proxy")

// ParseTrustedProxies parses a comma-separated list of IP addresses and
// CIDR prefixes, such as "10.0.0.0/8, 192.0.2.10".
func ParseTrustedProxies(list string) ([]netip.Prefix, error) {
	var (
		result []netip.Prefix
		errs   []error
	)

	for item := range strings.SplitSeq(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		// netip.ParseAddr would read "fe80::%eth0/64" as an address with
		// the zone "eth0/64", so only items without a "/" are addresses.
		if !strings.Contains(item, "/") {
			addr, err := netip.ParseAddr(item)
			if err != nil {
				errs = append(errs, fmt.Errorf("%w %q: %w", ErrInvalidTrustedProxy, item, err))
				continue
			}
			addr = addr.Unmap().WithZone("")
			result = append(result, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}

		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			errs = append(errs, fmt.Errorf("%w %q: %w", ErrInvalidTrustedProxy, item, err))
			continue
		}

		// ::ffff:10.0.0.0/104 would never match, because addresses are
		// unmapped before they are compared.
		if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}

		result = append(result, prefix.Masked())
	}

	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	return result, nil
}

// isTrustedProxy reports whether addr is a loopback address or inside one
// of the trusted prefixes.
func isTrustedProxy(addr netip.Addr, trustedProxies []netip.Prefix) bool {
	addr = addr.Unmap().WithZone("")
	if addr.IsLoopback() {
		return true
	}

	for _, prefix := range trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

// XForwardedForTrustedProxies replaces XForwardedForUpdate and
// XForwardedForToXRealIP when the reverse proxies in front of Anubis are
// known. It reduces X-Forwarded-For to the client address chosen by
// computeTrustedXFFHeader, and sets X-Real-IP to it when the proxy did not
// set X-Real-IP. Unlike those middlewares, it keeps private client
// addresses.
func XForwardedForTrustedProxies(trustedProxies []netip.Prefix, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Go reports a unix socket peer as "@", "" or a socket path,
		// depending on the OS. Any of them is a local proxy.
		remoteAddr := r.RemoteAddr
		if _, ok := r.Context().Value(http.LocalAddrContextKey).(*net.UnixAddr); ok {
			remoteAddr = "@"
		}

		// Some proxies, such as HAProxy, add their own X-Forwarded-For line
		// instead of appending to the client's. Header.Get would only return
		// the first, client-controlled, line.
		origXFFHeader := strings.Join(r.Header.Values("X-Forwarded-For"), ",")

		client, err := computeTrustedXFFHeader(remoteAddr, origXFFHeader, trustedProxies)
		if err != nil {
			// Never pass on a chain that was not checked.
			slog.DebugContext(r.Context(), "computing X-Forwarded-For header failed", "err", err)
			client = ""
		}

		if client == "" {
			r.Header.Del("X-Forwarded-For")
			next.ServeHTTP(w, r)
			return
		}

		r.Header.Set("X-Forwarded-For", client)

		if r.Header.Get("X-Real-IP") == "" {
			r.Header.Set("X-Real-IP", client)
			if addr, err := netip.ParseAddr(client); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), realIPKey{}, addr))
			}
		}

		next.ServeHTTP(w, r)
	})
}

// computeTrustedXFFHeader returns the client address of a request, like
// nginx real_ip_recursive and Rails trusted proxies do. It appends the
// remote address to the X-Forwarded-For chain and reads the chain from
// right to left, skipping empty entries and trusted proxies. The first
// address that is not trusted is the client, whatever its range. Entries
// to its left were written by the client and are ignored. If the chain
// ends or cannot be parsed before an untrusted address is found, the
// left-most trusted address read is used.
//
// A unix socket peer ("@") is a local proxy: it is trusted and not added
// to the chain.
func computeTrustedXFFHeader(remoteAddr string, origXFFHeader string, trustedProxies []netip.Prefix) (string, error) {
	var chain []string
	if origXFFHeader != "" {
		chain = strings.Split(origXFFHeader, ",")
	}

	if remoteAddr != "@" {
		remoteIP, _, err := net.SplitHostPort(remoteAddr)
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrCantSplitHostParse, err)
		}
		parsedRemoteIP, err := netip.ParseAddr(remoteIP)
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrCantParseRemoteIP, err)
		}
		chain = append(chain, parsedRemoteIP.String())
	}

	var furthest netip.Addr
	for i := len(chain) - 1; i >= 0; i-- {
		segment := strings.TrimSpace(chain[i])
		if segment == "" {
			continue
		}

		addr, err := parseXFFSegment(segment)
		if err != nil {
			slog.Debug("failed to parse XFF segment", "err", err)
			break
		}

		if !isTrustedProxy(addr, trustedProxies) {
			return addr.String(), nil
		}

		furthest = addr
	}

	if !furthest.IsValid() {
		return "", nil
	}

	return furthest.String(), nil
}

// parseXFFSegment parses an X-Forwarded-For entry. Some proxies add the
// client's port, as in "192.0.2.1:41000" or "[2001:db8::1]:41000".
func parseXFFSegment(segment string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(segment)
	if err == nil {
		return addr, nil
	}

	if addrPort, errPort := netip.ParseAddrPort(segment); errPort == nil {
		return addrPort.Addr(), nil
	}

	return netip.Addr{}, err
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

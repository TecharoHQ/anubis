package internal

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"time"

	proxyproto "github.com/pires/go-proxyproto"
)

// ProxyProtocolConfig holds configuration for PROXY protocol support.
type ProxyProtocolConfig struct {
	// Enabled enables PROXY protocol parsing on incoming connections.
	Enabled bool
	// Timeout is the maximum duration for reading the PROXY protocol header.
	// If zero, a default of 5 seconds is used.
	Timeout time.Duration
}

// WrapListenerWithProxyProtocol wraps a net.Listener to parse PROXY protocol headers.
// When enabled, it extracts the real client IP from the PROXY protocol header.
func WrapListenerWithProxyProtocol(ln net.Listener, config ProxyProtocolConfig) net.Listener {
	if !config.Enabled {
		return ln
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	slog.Info("enabling PROXY protocol support", "timeout", timeout)

	return &proxyproto.Listener{
		Listener:          ln,
		ReadHeaderTimeout: timeout,
	}
}

// ProxyProtocolXRealIP sets the X-Real-IP header based on the PROXY protocol
// source address if available.
func ProxyProtocolXRealIP(enabled bool, next http.Handler) http.Handler {
	if !enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn := r.Context().Value(proxyProtoConnKey{})
		if conn == nil {
			next.ServeHTTP(w, r)
			return
		}

		ppConn, ok := conn.(*proxyproto.Conn)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		header := ppConn.ProxyHeader()
		if header == nil {
			next.ServeHTTP(w, r)
			return
		}

		srcAddr := header.SourceAddr
		if srcAddr == nil {
			next.ServeHTTP(w, r)
			return
		}

		var ip string
		switch addr := srcAddr.(type) {
		case *net.TCPAddr:
			ip = addr.IP.String()
		case *net.UDPAddr:
			ip = addr.IP.String()
		default:
			host, _, err := net.SplitHostPort(srcAddr.String())
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ip = host
		}

		slog.Debug("setting X-Real-IP from PROXY protocol", "ip", ip)
		r.Header.Set("X-Real-IP", ip)

		if addr, err := netip.ParseAddr(ip); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), realIPKey{}, addr))
		}

		next.ServeHTTP(w, r)
	})
}

type proxyProtoConnKey struct{}

// StoreProxyProtoConn is a middleware that stores the underlying connection in the request context
// for later extraction of PROXY protocol information.
func StoreProxyProtoConn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The connection is already stored by the ConnContext in the http.Server
		next.ServeHTTP(w, r)
	})
}

// ProxyProtoConnContext returns a ConnContext function that stores the connection
// in the request context for PROXY protocol header extraction.
func ProxyProtoConnContext(enabled bool) func(ctx context.Context, c net.Conn) context.Context {
	if !enabled {
		return nil
	}
	return func(ctx context.Context, c net.Conn) context.Context {
		return context.WithValue(ctx, proxyProtoConnKey{}, c)
	}
}

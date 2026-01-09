package internal

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/pires/go-proxyproto"
)

type proxyProtocolUsedKey struct{}

func ProxyProtocolUsed(ctx context.Context) bool {
	v, ok := ctx.Value(proxyProtocolUsedKey{}).(bool)
	return ok && v
}

type proxyProtocolHeaderKey struct{}

func ProxyProtocolHeader(ctx context.Context) (*proxyproto.Header, bool) {
	h, ok := ctx.Value(proxyProtocolHeaderKey{}).(*proxyproto.Header)
	return h, ok
}

type proxyProtocolInfoKey struct{}

func proxyProtocolInfo(ctx context.Context) (ProxyProtocolInfo, bool) {
	h, ok := ctx.Value(proxyProtocolInfoKey{}).(ProxyProtocolInfo)
	return h, ok
}

type ProxyProtocolInfo struct {
	AddrPort netip.AddrPort
}

func ProxyProtoConnContext() func(ctx context.Context, c net.Conn) context.Context {
	return func(ctx context.Context, c net.Conn) context.Context {
		ppConn, ok := c.(*proxyproto.Conn)
		if !ok {
			return ctx
		}

		hdr := ppConn.ProxyHeader()
		if hdr == nil {
			return context.WithValue(ctx, proxyProtocolUsedKey{}, false)
		}

		ctx = context.WithValue(ctx, proxyProtocolUsedKey{}, true)
		ctx = context.WithValue(ctx, proxyProtocolHeaderKey{}, hdr)

		return ctx
	}
}

func SendProxyProtocolDialer(dialer *net.Dialer, proxyProtocolVersion string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, fmt.Errorf("dial error: %w", err)
		}
		// stolen from caddyserver :)
		proxyProtocolInfo, ok := proxyProtocolInfo(ctx)
		if !ok {
			return nil, fmt.Errorf("failed to get proxy protocol info from context")
		}
		var proxyv byte
		switch proxyProtocolVersion {
		case "v1":
			proxyv = 1
		case "v2":
			proxyv = 2
		default:
			return nil, fmt.Errorf("unexpected proxy protocol version")
		}

		// The src and dst have to be of the same address family. As we don't know the original
		// dst address (it's kind of impossible to know) and this address is generally of very
		// little interest, we just set it to all zeros.
		var destAddr net.Addr
		switch {
		case proxyProtocolInfo.AddrPort.Addr().Is4():
			destAddr = &net.TCPAddr{
				IP: net.IPv4zero,
			}
		case proxyProtocolInfo.AddrPort.Addr().Is6():
			destAddr = &net.TCPAddr{
				IP: net.IPv6zero,
			}
		default:
			return nil, fmt.Errorf("unexpected remote addr type in proxy protocol info")
		}
		sourceAddr := &net.TCPAddr{
			IP:   proxyProtocolInfo.AddrPort.Addr().AsSlice(),
			Port: int(proxyProtocolInfo.AddrPort.Port()),
			Zone: proxyProtocolInfo.AddrPort.Addr().Zone(),
		}
		header := proxyproto.HeaderProxyFromAddrs(proxyv, sourceAddr, destAddr)

		_, err = header.WriteTo(conn)

		if err != nil {
			return nil, err
		}

		return conn, nil
	}
}

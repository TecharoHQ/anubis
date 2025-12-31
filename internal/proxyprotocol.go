package internal

import (
	"context"
	"net"

	"github.com/pires/go-proxyproto"
)

type proxyProtocolUsedKey struct{}

func ProxyProtocolUsed(ctx context.Context) bool {
	v, ok := ctx.Value(proxyProtocolUsedKey{}).(bool)
	return ok && v
}

type proxyProtocolHeaderKey struct{}

func ProxyProtocolHeader(ctx context.Context) (proxyproto.Header, bool) {
	v, ok := ctx.Value(proxyProtocolUsedKey{}).(proxyproto.Header)
	return v, ok
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

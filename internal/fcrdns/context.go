package fcrdns

import "context"

type ctxKey struct{}

func With(ctx context.Context, fcrdns *FCrDNS) context.Context {
	return context.WithValue(ctx, ctxKey{}, fcrdns)
}

func FromContext(ctx context.Context) (*FCrDNS, bool) {
	cli, ok := ctx.Value(ctxKey{}).(*FCrDNS)
	return cli, ok
}

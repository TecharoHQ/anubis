package thoth

import "context"

type ctxKey struct{}

func With(ctx context.Context, cli *Client) context.Context {
	return context.WithValue(ctx, ctxKey{}, cli)
}

func FromContext(ctx context.Context) (*Client, bool) {
	cli, ok := ctx.Value(ctxKey{}).(*Client)
	return cli, ok
}

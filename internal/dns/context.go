package dns

import "context"

type ctxKey struct{}

// With returns a context that carries the given DNS resolver. Checkers built
// from a context derived from this one observe it via [FromContext].
func With(ctx context.Context, d *Dns) context.Context {
	return context.WithValue(ctx, ctxKey{}, d)
}

// FromContext returns the DNS resolver stored on ctx, if any.
func FromContext(ctx context.Context) (*Dns, bool) {
	d, ok := ctx.Value(ctxKey{}).(*Dns)
	return d, ok
}

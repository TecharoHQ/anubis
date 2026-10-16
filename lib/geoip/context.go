package geoip

import "context"

type ctxKey struct{}

// With attaches db to ctx. A geoip block in the policy file takes precedence
// over a DB passed this way.
func With(ctx context.Context, db *DB) context.Context {
	return context.WithValue(ctx, ctxKey{}, db)
}

func FromContext(ctx context.Context) (*DB, bool) {
	db, ok := ctx.Value(ctxKey{}).(*DB)
	return db, ok && db != nil
}

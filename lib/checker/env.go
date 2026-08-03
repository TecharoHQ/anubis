package checker

import "context"

// subrequestModeKey is the context key under which subrequest mode is stored.
type subrequestModeKey struct{}

// WithSubrequestMode returns a context that carries the given subrequest mode
// flag. Checkers built from a context derived from this one observe the flag
// via [SubrequestMode].
//
// Subrequest mode is a deployment-wide flag (it is the same for every checker
// in an Anubis instance), so it is threaded through the build context rather
// than the per-checker configuration.
func WithSubrequestMode(ctx context.Context, on bool) context.Context {
	return context.WithValue(ctx, subrequestModeKey{}, on)
}

// SubrequestMode reports whether subrequest mode is enabled on ctx. It defaults
// to false when the flag has not been set.
func SubrequestMode(ctx context.Context) bool {
	on, _ := ctx.Value(subrequestModeKey{}).(bool)
	return on
}

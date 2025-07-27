package thothmock

import (
	"context"
	"testing"

	"github.com/TecharoHQ/anubis/lib/thoth"
)

func WithMockThoth(t *testing.T) context.Context {
	t.Helper()

	thothCli := &thoth.Client{}
	thothCli.WithIPToASNService(MockIpToASNService())
	ctx := thoth.With(t.Context(), thothCli)
	return ctx
}

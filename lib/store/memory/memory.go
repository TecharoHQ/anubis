package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/TecharoHQ/anubis/decaymap"
	"github.com/TecharoHQ/anubis/lib/store"
)

type factory struct{}

func (factory) Build(ctx context.Context, _ json.RawMessage) (store.Interface, error) {
	return New(ctx), nil
}

func (factory) Valid(json.RawMessage) error { return nil }

func init() {
	store.Register("memory", factory{})
}

type impl struct {
	store *decaymap.Impl[string, []byte]
}

func (i *impl) Delete(_ context.Context, key string) error {
	if !i.store.Delete(key) {
		return fmt.Errorf("%w: %q", store.ErrNotFound, key)
	}

	return nil
}

func (i *impl) Get(_ context.Context, key string) ([]byte, error) {
	result, ok := i.store.Get(key)
	if !ok {
		return nil, fmt.Errorf("%w: %q", store.ErrNotFound, key)
	}

	return result, nil
}

func (i *impl) Set(_ context.Context, key string, value []byte, expiry time.Duration) error {
	i.store.Set(key, value, expiry)
	return nil
}

func (i *impl) cleanupThread(ctx context.Context) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			i.store.Cleanup()
		}
	}
}

// New creates a simple in-memory store. This will not scale to multiple Anubis instances.
func New(ctx context.Context) store.Interface {
	result := &impl{
		store: decaymap.New[string, []byte](),
	}

	go result.cleanupThread(ctx)

	return result
}

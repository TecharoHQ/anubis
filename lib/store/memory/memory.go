package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
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

const maxEntries = 10000

type impl struct {
	lock      sync.Mutex
	slots     [maxEntries]string
	positions map[string]int
	next      int
	store     *decaymap.Impl[string, []byte]
}

func (i *impl) Delete(_ context.Context, key string) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	delete(i.positions, key)
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
	i.lock.Lock()
	defer i.lock.Unlock()
	if _, exists := i.positions[key]; !exists {
		old := i.slots[i.next]
		if index, exists := i.positions[old]; exists && index == i.next {
			i.store.Delete(old)
			delete(i.positions, old)
		}
		i.slots[i.next] = key
		i.positions[key] = i.next
		i.next = (i.next + 1) % maxEntries
	}
	i.store.Set(key, value, expiry)
	return nil
}

func (i *impl) IsPersistent() bool {
	return false
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
		store:     decaymap.New[string, []byte](),
		positions: make(map[string]int),
	}

	go result.cleanupThread(ctx)

	return result
}

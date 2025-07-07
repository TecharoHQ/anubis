package store

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
)

var (
	registry map[string]Factory = map[string]Factory{}
	regLock  sync.RWMutex
)

type Factory interface {
	Build(ctx context.Context, config json.RawMessage) (Interface, error)
	Valid(config json.RawMessage) error
}

func Register(name string, impl Factory) {
	regLock.Lock()
	defer regLock.Unlock()

	registry[name] = impl
}

func Get(name string) (Factory, bool) {
	regLock.RLock()
	defer regLock.RUnlock()
	result, ok := registry[name]
	return result, ok
}

func Methods() []string {
	regLock.RLock()
	defer regLock.RUnlock()
	var result []string
	for method := range registry {
		result = append(result, method)
	}
	sort.Strings(result)
	return result
}

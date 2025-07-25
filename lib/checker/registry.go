package checker

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
)

type Factory interface {
	Build(context.Context, json.RawMessage) (Interface, error)
	Valid(context.Context, json.RawMessage) error
}

var (
	registry map[string]Factory = map[string]Factory{}
	regLock  sync.RWMutex
)

func Register(name string, factory Factory) {
	regLock.Lock()
	defer regLock.Unlock()

	registry[name] = factory
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

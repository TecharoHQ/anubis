package challenge

import (
	"log/slog"
	"net/http"
	"sort"
	"sync"

	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/a-h/templ"
)

var (
	registry map[string]Impl = map[string]Impl{}
	regLock  sync.RWMutex
)

func Register(name string, impl Impl) {
	regLock.Lock()
	defer regLock.Unlock()

	registry[name] = impl
}

func Get(name string) (Impl, bool) {
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

type Impl interface {
	Fail(w http.ResponseWriter, r *http.Request) error
	Issue(r *http.Request, lg *slog.Logger, rule *policy.Bot, challenge string, ogTags map[string]string) (templ.Component, error)
	Validate(r *http.Request, lg *slog.Logger, rule *policy.Bot, challenge string) error
}

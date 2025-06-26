package challenge

import (
	"log/slog"
	"net/http"
	"sort"
	"sync"

	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/policy/config"
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

type IssueInput struct {
	Impressum *config.Impressum
	Rule      *policy.Bot
	Challenge string
	OGTags    map[string]string
}

type Impl interface {
	// Setup registers any additional routes with the Impl for assets or API routes.
	Setup(mux *http.ServeMux)

	// Issue a new challenge to the user, called by the Anubis.
	Issue(r *http.Request, lg *slog.Logger, in *IssueInput) (templ.Component, error)

	// Validate a challenge, making sure that it passes muster.
	Validate(r *http.Request, lg *slog.Logger, rule *policy.Bot, challenge string) error
}

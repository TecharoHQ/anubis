package extension

import (
	"log/slog"
	"net/http"
	"sort"
	"sync"

	"github.com/TecharoHQ/anubis/lib/challenge"
	"github.com/TecharoHQ/anubis/lib/store"
	"github.com/a-h/templ"
)

var (
	registry = map[string]Impl{}
	regLock  sync.RWMutex
)

// Register adds an extension to the global extension registry.
func Register(name string, impl Impl) {
	regLock.Lock()
	defer regLock.Unlock()

	registry[name] = impl
}

// Get returns the extension by name. If no extension is found, ok is false.
func Get(name string) (impl Impl, ok bool) {
	regLock.RLock()
	defer regLock.RUnlock()
	result, ok := registry[name]
	return result, ok
}

// Names returns the names of all loaded challenge extensions.
func Names() []string {
	regLock.RLock()
	defer regLock.RUnlock()
	result := make([]string, 0, len(registry))
	for name := range registry {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// Impl is the implementation of a given challenge extension.
type Impl interface {
	// Setup registers any relevant extension routes. Routes MUST include
	// anubis.BasePrefix.
	Setup(mux *http.ServeMux, st store.Interface) error

	// Head returns additional data that is put into the HTML <head> block
	// on the challenge page.
	//
	// Scripts rendered here run before challenge scripts (if any).
	Head(r *http.Request, chall *challenge.Challenge) templ.Component

	// Validate ensures that this challenge method's side effects have been
	// fulfilled. Returning a non-nil error refuses to mint a JWT.
	//
	// If you return *challenge.Error via challenge.NewError, the client
	// gets a more useful message.
	Validate(r *http.Request, lg *slog.Logger, in *challenge.ValidateInput) error
}

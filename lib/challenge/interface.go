package challenge

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/store"
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
	Challenge *Challenge
	OGTags    map[string]string
	Store     store.Interface
}

func (in *IssueInput) Valid() error {
	if in == nil {
		return fmt.Errorf("%w: IssueInput is nil", ErrInvalidInput)
	}
	if in.Rule == nil {
		return fmt.Errorf("%w: Rule is nil", ErrInvalidInput)
	}
	if in.Rule.Challenge == nil {
		return fmt.Errorf("%w: Rule.Challenge is nil", ErrInvalidInput)
	}
	if in.Challenge == nil {
		return fmt.Errorf("%w: Challenge is nil", ErrInvalidInput)
	}
	return nil
}

type ValidateInput struct {
	Rule      *policy.Bot
	Challenge *Challenge
	Store     store.Interface
}

func (in *ValidateInput) Valid() error {
	if in == nil {
		return fmt.Errorf("%w: ValidateInput is nil", ErrInvalidInput)
	}
	if in.Rule == nil {
		return fmt.Errorf("%w: Rule is nil", ErrInvalidInput)
	}
	if in.Rule.Challenge == nil {
		return fmt.Errorf("%w: Rule.Challenge is nil", ErrInvalidInput)
	}
	if in.Challenge == nil {
		return fmt.Errorf("%w: Challenge is nil", ErrInvalidInput)
	}
	return nil
}

type Impl interface {
	// Setup registers any additional routes with the Impl for assets or API routes.
	Setup(mux *http.ServeMux)

	// Issue a new challenge to the user, called by the Anubis.
	Issue(w http.ResponseWriter, r *http.Request, lg *slog.Logger, in *IssueInput) (templ.Component, error)

	// Validate a challenge, making sure that it passes muster.
	Validate(r *http.Request, lg *slog.Logger, in *ValidateInput) error
}

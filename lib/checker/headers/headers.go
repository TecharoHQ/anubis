package headers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/checker/headerexists"
	"github.com/TecharoHQ/anubis/lib/checker/headermatches"
	"github.com/TecharoHQ/anubis/lib/config"
)

var (
	ErrInvalidConfig = errors.New("checker.headers: invalid config")
	ErrNoHeaders     = errors.New("checker.headers: no headers supplied")
)

// existsValue is the magic regexp value that means "this header simply has to
// exist" instead of "this header has to match a regexp".
const existsValue = ".*"

func init() {
	checker.Register("headers", Factory{})
}

var (
	existsFactory  = headerexists.Factory{}
	matchesFactory = headermatches.Factory{}
)

type Factory struct{}

func (Factory) ValidateConfig(ctx context.Context, inp json.RawMessage) error {
	var c Config
	if err := json.Unmarshal([]byte(inp), &c); err != nil {
		return fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	if err := c.Valid(ctx); err != nil {
		return err
	}

	return nil
}

func (Factory) Create(ctx context.Context, inp json.RawMessage) (checker.Impl, error) {
	var c Config
	if err := json.Unmarshal([]byte(inp), &c); err != nil {
		return nil, fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	var result checker.All
	var errs []error

	for _, header := range c.keys() {
		fac, subInp, err := c.sub(header)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		impl, err := fac.Create(ctx, subInp)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		result = append(result, impl)
	}

	if len(errs) != 0 {
		return nil, errors.Join(append([]error{ErrInvalidConfig}, errs...)...)
	}

	return &Impl{
		checkers: result,
		hash:     internal.SHA256sum(string(inp)),
	}, nil
}

// Config maps header names to a regexp that their value must match. The magic
// value ".*" means the header only has to be present (delegated to the
// header_exists checker), anything else is delegated to the header_matches
// checker.
type Config map[string]string

// keys returns the header names in sorted order so that construction and
// hashing are deterministic regardless of map iteration order.
func (c Config) keys() []string {
	keys := make([]string, 0, len(c))
	for header := range c {
		keys = append(keys, header)
	}
	sort.Strings(keys)
	return keys
}

// sub returns the checker factory and the marshalled sub-config to build the
// checker responsible for the given header.
func (c Config) sub(header string) (checker.Factory, json.RawMessage, error) {
	value := c[header]

	if strings.TrimSpace(value) == existsValue {
		data, err := json.Marshal(headerexists.Config{Header: header})
		if err != nil {
			return nil, nil, fmt.Errorf("[unexpected] can't marshal header_exists config for %q: %w", header, err)
		}
		return existsFactory, data, nil
	}

	data, err := json.Marshal(headermatches.Config{Header: header, Regexp: value})
	if err != nil {
		return nil, nil, fmt.Errorf("[unexpected] can't marshal header_matches config for %q: %w", header, err)
	}
	return matchesFactory, data, nil
}

func (c Config) Valid(ctx context.Context) error {
	var errs []error

	if len(c) == 0 {
		errs = append(errs, ErrNoHeaders)
	}

	for _, header := range c.keys() {
		fac, subInp, err := c.sub(header)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if err := fac.ValidateConfig(ctx, subInp); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errors.Join(append([]error{ErrInvalidConfig}, errs...)...)
	}

	return nil
}

type Impl struct {
	checkers checker.All
	hash     string
}

func (i *Impl) Check(r *http.Request) (bool, error) { return i.checkers.Check(r) }

func (i *Impl) Hash() string { return i.hash }

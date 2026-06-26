package headerexists

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/config"
)

var (
	ErrInvalidConfig      = errors.New("checker.header_exists: invalid config")
	ErrNoHeader           = errors.New("checker.header_exists: no header supplied")
	ErrHeaderNotCanonical = errors.New("checker.header_exists: header is not in Canonical-Http-Header-Form")
)

func init() {
	checker.Register("header_exists", Factory{})
}

type Factory struct{}

func (Factory) ValidateConfig(ctx context.Context, inp json.RawMessage) error {
	var fc Config
	if err := json.Unmarshal([]byte(inp), &fc); err != nil {
		return fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	if err := fc.Valid(); err != nil {
		return err
	}

	return nil
}

func (Factory) Create(ctx context.Context, inp json.RawMessage) (checker.Impl, error) {
	var fc Config

	if err := json.Unmarshal([]byte(inp), &fc); err != nil {
		return nil, fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	return &Impl{
		header: fc.Header,
		hash:   internal.SHA256sum(string(inp)),
	}, nil
}

type Config struct {
	Header string `json:"header"`
}

func (fc Config) Valid() error {
	var errs []error

	if fc.Header == "" {
		errs = append(errs, ErrNoHeader)
	}

	if http.CanonicalHeaderKey(fc.Header) != fc.Header {
		errs = append(errs, ErrHeaderNotCanonical)
	}

	if len(errs) != 0 {
		return errors.Join(append([]error{ErrInvalidConfig}, errs...)...)
	}

	return nil
}

type Impl struct {
	header string
	hash   string
}

func (i *Impl) Check(r *http.Request) (bool, error) {
	return r.Header.Get(i.header) != "", nil
}

func (i *Impl) Hash() string { return i.hash }

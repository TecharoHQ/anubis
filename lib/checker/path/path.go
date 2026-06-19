package path

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/config"
)

var (
	ErrInvalidConfig = errors.New("checker.path: invalid config")
	ErrNoRegexp      = errors.New("checker.path: no regexp supplied")
	ErrInvalidRegexp = errors.New("checker.path: invalid regexp")
)

func init() {
	checker.Register("path", Factory{})
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

func (f Factory) Create(ctx context.Context, inp json.RawMessage) (checker.Impl, error) {
	var fc Config

	if err := json.Unmarshal([]byte(inp), &fc); err != nil {
		return nil, fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	rex, err := regexp.Compile(strings.TrimSpace(fc.Regexp))
	if err != nil {
		return nil, fmt.Errorf("[unexpected] %w: %w", ErrInvalidRegexp, err)
	}

	return &Impl{
		regexp:     rex,
		subrequest: checker.SubrequestMode(ctx),
		hash:       internal.SHA256sum(string(inp)),
	}, nil
}

type Config struct {
	Regexp string `json:"matches"`
}

func (fc Config) Valid() error {
	var errs []error

	if fc.Regexp == "" {
		errs = append(errs, ErrNoRegexp)
	}

	if _, err := regexp.Compile(strings.TrimSpace(fc.Regexp)); err != nil {
		errs = append(errs, fmt.Errorf("%w: %w", ErrInvalidRegexp, err))
	}

	if len(errs) != 0 {
		return errors.Join(append([]error{ErrInvalidConfig}, errs...)...)
	}

	return nil
}

type Impl struct {
	regexp     *regexp.Regexp
	subrequest bool
	hash       string
}

func (i *Impl) Check(r *http.Request) (bool, error) {
	if i.subrequest {
		originalUrl := r.Header.Get("X-Original-URI")
		if originalUrl == "" {
			originalUrl = r.Header.Get("X-Forwarded-Uri")
		}
		if originalUrl != "" {
			if parsed, err := url.ParseRequestURI(originalUrl); err == nil {
				originalUrl = parsed.Path
			}
			if i.regexp.MatchString(originalUrl) {
				return true, nil
			}
		}
	}

	return i.regexp.MatchString(r.URL.Path), nil
}

func (i *Impl) Hash() string { return i.hash }

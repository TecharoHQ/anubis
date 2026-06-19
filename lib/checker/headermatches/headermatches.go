package headermatches

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker"
	"github.com/TecharoHQ/anubis/lib/config"
)

var (
	ErrInvalidConfig       = errors.New("checker.header_matches: invalid config")
	ErrNoHeaderName        = errors.New("checker.header_matches: no header name supplied")
	ErrHeaderNotCanonical  = errors.New("checker.header_matches: header is not in Canonical-Http-Header-Form")
	ErrNoHeaderRegexp      = errors.New("checker.header_matches: no header value regexp supplied")
	ErrInvalidHeaderRegexp = errors.New("checker.header_matches: invalid header value regexp supplied")
)

func init() {
	checker.Register("header_matches", Factory{})
}

type Factory struct{}

func (Factory) ValidateConfig(inp json.RawMessage) error {
	var fc fileConfig
	if err := json.Unmarshal([]byte(inp), &fc); err != nil {
		return fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	if err := fc.Valid(); err != nil {
		return err
	}

	return nil
}

func (Factory) Create(inp json.RawMessage) (checker.Impl, error) {
	var fc fileConfig

	if err := json.Unmarshal([]byte(inp), &fc); err != nil {
		return nil, fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	rex, err := regexp.Compile(strings.TrimSpace(fc.Regexp))
	if err != nil {
		return nil, fmt.Errorf("[unexpected] %w: %w", ErrInvalidHeaderRegexp, err)
	}

	return &Impl{
		name:   fc.Header,
		regexp: rex,
		hash:   internal.SHA256sum(string(inp)),
	}, nil
}

type fileConfig struct {
	Header string `json:"header"`
	Regexp string `json:"regexp"`
}

func (fc fileConfig) Valid() error {
	var errs []error

	if fc.Header == "" {
		errs = append(errs, ErrNoHeaderName)
	}

	if http.CanonicalHeaderKey(fc.Header) != fc.Header {
		errs = append(errs, ErrHeaderNotCanonical)
	}

	if fc.Regexp == "" {
		errs = append(errs, ErrNoHeaderRegexp)
	}

	if _, err := regexp.Compile(strings.TrimSpace(fc.Regexp)); err != nil {
		errs = append(errs, fmt.Errorf("%w: %w", ErrInvalidHeaderRegexp, err))
	}

	if len(errs) != 0 {
		return errors.Join(append([]error{ErrInvalidConfig}, errs...)...)
	}

	return nil
}

type Impl struct {
	name   string
	regexp *regexp.Regexp
	hash   string
}

func (i *Impl) Check(r *http.Request) (bool, error) {
	return i.regexp.MatchString(r.Header.Get(i.name)), nil
}

func (i *Impl) Hash() string { return i.hash }

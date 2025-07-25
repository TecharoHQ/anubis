package headermatches

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker"
)

func init() {
	checker.Register("header_matches", Factory{})
	checker.Register("user_agent", Factory{defaultHeader: "User-Agent"})
}

type Factory struct {
	defaultHeader string
}

func (f Factory) Build(ctx context.Context, data json.RawMessage) (checker.Interface, error) {
	var fc fileConfig

	if f.defaultHeader != "" {
		fc.Header = f.defaultHeader
	}

	if err := json.Unmarshal([]byte(data), &fc); err != nil {
		return nil, errors.Join(checker.ErrUnparseableConfig, err)
	}

	if err := fc.Valid(); err != nil {
		return nil, errors.Join(checker.ErrInvalidConfig, err)
	}

	valueRex, err := regexp.Compile(fc.ValueRegex)
	if err != nil {
		return nil, errors.Join(ErrInvalidRegex, err)
	}

	return &Checker{
		header: http.CanonicalHeaderKey(fc.Header),
		regexp: valueRex,
		hash:   internal.FastHash(fc.String()),
	}, nil
}

func (f Factory) Valid(ctx context.Context, data json.RawMessage) error {
	var fc fileConfig

	if f.defaultHeader != "" {
		fc.Header = f.defaultHeader
	}

	if err := json.Unmarshal([]byte(data), &fc); err != nil {
		return err
	}

	if err := fc.Valid(); err != nil {
		return err
	}

	return nil
}

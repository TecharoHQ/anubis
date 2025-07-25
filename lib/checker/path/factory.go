package path

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/checker"
)

func init() {
	checker.Register("path", Factory{})
}

type Factory struct{}

func (f Factory) Build(ctx context.Context, data json.RawMessage) (checker.Interface, error) {
	var fc fileConfig

	if err := json.Unmarshal([]byte(data), &fc); err != nil {
		return nil, errors.Join(checker.ErrUnparseableConfig, err)
	}

	if err := fc.Valid(); err != nil {
		return nil, errors.Join(checker.ErrInvalidConfig, err)
	}

	pathRex, err := regexp.Compile(strings.TrimSpace(fc.Regex))
	if err != nil {
		return nil, errors.Join(ErrInvalidRegex, err)
	}

	return &Checker{
		regexp: pathRex,
		hash:   internal.FastHash(fc.String()),
	}, nil
}

func (f Factory) Valid(ctx context.Context, data json.RawMessage) error {
	var fc fileConfig

	if err := json.Unmarshal([]byte(data), &fc); err != nil {
		return errors.Join(checker.ErrUnparseableConfig, err)
	}

	return fc.Valid()
}

func Valid(pathRex string) error {
	fc := fileConfig{
		Regex: pathRex,
	}

	return fc.Valid()
}

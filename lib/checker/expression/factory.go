package expression

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/TecharoHQ/anubis/lib/checker"
)

func init() {
	checker.Register("expression", Factory{})
}

type Factory struct{}

func (f Factory) Build(ctx context.Context, data json.RawMessage) (checker.Interface, error) {
	var fc = &Config{}

	if err := json.Unmarshal([]byte(data), fc); err != nil {
		return nil, errors.Join(checker.ErrUnparseableConfig, err)
	}

	if err := fc.Valid(); err != nil {
		return nil, errors.Join(checker.ErrInvalidConfig, err)
	}

	return New(fc)
}

func (f Factory) Valid(ctx context.Context, data json.RawMessage) error {
	var fc = &Config{}

	if err := json.Unmarshal([]byte(data), fc); err != nil {
		return err
	}

	if err := fc.Valid(); err != nil {
		return err
	}

	return nil
}

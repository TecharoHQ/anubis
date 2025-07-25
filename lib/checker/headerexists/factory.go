package headerexists

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/TecharoHQ/anubis/lib/checker"
)

type Factory struct{}

func (f Factory) Build(ctx context.Context, data json.RawMessage) (checker.Interface, error) {
	var headerName string

	if err := json.Unmarshal([]byte(data), &headerName); err != nil {
		return nil, fmt.Errorf("%w: want string", checker.ErrUnparseableConfig)
	}

	if err := f.Valid(ctx, data); err != nil {
		return nil, err
	}

	return New(http.CanonicalHeaderKey(headerName)), nil
}

func (Factory) Valid(ctx context.Context, data json.RawMessage) error {
	var headerName string

	if err := json.Unmarshal([]byte(data), &headerName); err != nil {
		return fmt.Errorf("%w: want string", checker.ErrUnparseableConfig)
	}

	if headerName == "" {
		return fmt.Errorf("%w: string must not be empty", checker.ErrInvalidConfig)
	}

	return nil
}

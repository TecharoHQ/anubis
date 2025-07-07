package valkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TecharoHQ/anubis/lib/store"
	valkey "github.com/redis/go-redis/v9"
)

var (
	ErrNoURL  = errors.New("valkey.Config: no URL defined")
	ErrBadURL = errors.New("valkey.Config: URL is invalid")
)

func init() {
	store.Register("valkey", Factory{})
}

type Factory struct{}

func (Factory) Build(ctx context.Context, data json.RawMessage) (store.Interface, error) {
	var config Config

	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	if err := config.Valid(); err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	opts, err := valkey.ParseURL(config.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	rdb := valkey.NewClient(opts)

	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("can't ping valkey instance: %w", err)
	}

	return &Store{
		rdb: rdb,
	}, nil
}

func (Factory) Valid(data json.RawMessage) error {
	var config Config
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	if err := config.Valid(); err != nil {
		return fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	return nil
}

type Config struct {
	URL string `json:"url"`
}

func (c Config) Valid() error {
	var errs []error

	if c.URL == "" {
		errs = append(errs, ErrNoURL)
	}

	if _, err := valkey.ParseURL(c.URL); err != nil {
		errs = append(errs, ErrBadURL)
	}

	if len(errs) != 0 {
		return fmt.Errorf("valkey.Config: invalid config: %w", errors.Join(errs...))
	}

	return nil
}

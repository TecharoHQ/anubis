package valkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/TecharoHQ/anubis/lib/store"
	valkey "github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

func init() {
	store.Register("valkey", Factory{})
}

// Errors kept as-is so other code/tests still pass.
var (
	ErrNoURL  = errors.New("valkey.Config: no URL defined")
	ErrBadURL = errors.New("valkey.Config: URL is invalid")
)

// Config is what Anubis unmarshals from the "parameters" JSON.
type Config struct {
	URL     string `json:"url"`
	Cluster bool   `json:"cluster,omitempty"`
}

func (c Config) Valid() error {
	if c.URL == "" {
		return ErrNoURL
	}

	// Just validate that it's a valid Redis URL.
	if _, err := valkey.ParseURL(c.URL); err != nil {
		return fmt.Errorf("%w: %v", ErrBadURL, err)
	}

	return nil
}

// redisClient is satisfied by *valkey.Client and *valkey.ClusterClient.
type redisClient interface {
	Get(ctx context.Context, key string) *valkey.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *valkey.StatusCmd
	Del(ctx context.Context, keys ...string) *valkey.IntCmd
	Ping(ctx context.Context) *valkey.StatusCmd
}

type Factory struct{}

func (Factory) Valid(data json.RawMessage) error {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	return cfg.Valid()
}

func (Factory) Build(ctx context.Context, data json.RawMessage) (store.Interface, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Valid(); err != nil {
		return nil, err
	}

	opts, err := valkey.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("valkey.Factory: %w", err)
	}

	var client redisClient

	if cfg.Cluster {
		// Cluster mode: use the parsed Addr as the seed node.
		clusterOpts := &valkey.ClusterOptions{
			Addrs: []string{opts.Addr},
			// Explicitly disable maintenance notifications
			// This prevents the client from sending CLIENT MAINT_NOTIFICATIONS ON
			MaintNotificationsConfig: &maintnotifications.Config{
				Mode: maintnotifications.ModeDisabled,
			},
		}
		client = valkey.NewClusterClient(clusterOpts)
	} else {
		opts.MaintNotificationsConfig = &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		}
		client = valkey.NewClient(opts)
	}

	// Optional but nice: fail fast if the cluster/single node is unreachable.
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("valkey.Factory: ping failed: %w", err)
	}

	return &Store{client: client}, nil
}

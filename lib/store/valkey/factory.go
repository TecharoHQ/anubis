package valkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/store"
	valkey "github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

func init() {
	store.Register("valkey", Factory{})
}

var (
	ErrNoURL  = errors.New("valkey.Config: no URL defined")
	ErrBadURL = errors.New("valkey.Config: URL is invalid")

	// Sentinel validation errors
	ErrSentinelMasterNameRequired = errors.New("valkey.Sentinel: masterName is required")
	ErrSentinelAddrRequired       = errors.New("valkey.Sentinel: addr is required")
	ErrSentinelAddrEmpty          = errors.New("valkey.Sentinel: addr cannot be empty")
)

// Config is what Anubis unmarshals from the "parameters" JSON.
type Config struct {
	URL     string `json:"url"`
	Cluster bool   `json:"cluster,omitempty"`

	Sentinel *Sentinel `json:"sentinel,omitempty"`
}

func (c Config) Valid() error {
	var errs []error

	if c.URL == "" && c.Sentinel == nil {
		errs = append(errs, ErrNoURL)
	}

	// Validate URL only if provided
	if c.URL != "" {
		if _, err := valkey.ParseURL(c.URL); err != nil {
			errs = append(errs, fmt.Errorf("%w: %v", ErrBadURL, err))
		}
	}

	if c.Sentinel != nil {
		if err := c.Sentinel.Valid(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

type Sentinel struct {
	MasterName string                  `json:"masterName"`
	Addr       internal.ListOr[string] `json:"addr"`
	ClientName string                  `json:"clientName,omitempty"`
	Username   string                  `json:"username,omitempty"`
	Password   string                  `json:"password,omitempty"`
}

func (s Sentinel) Valid() error {
	var errs []error

	if s.MasterName == "" {
		errs = append(errs, ErrSentinelMasterNameRequired)
	}

	if len(s.Addr) == 0 {
		errs = append(errs, ErrSentinelAddrRequired)
	} else {
		// Check if all addresses in the list are empty
		allEmpty := true
		for _, addr := range s.Addr {
			if addr != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			errs = append(errs, ErrSentinelAddrEmpty)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
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

	var client redisClient

	switch {
	case cfg.Cluster:
		opts, err := valkey.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("valkey.Factory: %w", err)
		}

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
	case cfg.Sentinel != nil:
		opts := &valkey.FailoverOptions{
			MasterName:       cfg.Sentinel.MasterName,
			SentinelAddrs:    cfg.Sentinel.Addr,
			SentinelUsername: cfg.Sentinel.Username,
			SentinelPassword: cfg.Sentinel.Password,
			Username:         cfg.Sentinel.Username,
			Password:         cfg.Sentinel.Password,
			ClientName:       cfg.Sentinel.ClientName,
		}
		client = valkey.NewFailoverClusterClient(opts)
	default:
		opts, err := valkey.ParseURL(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("valkey.Factory: %w", err)
		}

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

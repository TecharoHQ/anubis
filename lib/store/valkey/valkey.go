package valkey

import (
	"context"
	"fmt"
	"time"

	"github.com/TecharoHQ/anubis/lib/store"
	valkey "github.com/redis/go-redis/v9"
)

type Store struct {
	rdb *valkey.Client
}

func (s *Store) Delete(ctx context.Context, key string) error {
	n, err := s.rdb.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("can't delete from valkey: %w", err)
	}

	switch n {
	case 0:
		return fmt.Errorf("%w: %d key(s) deleted", store.ErrNotFound, n)
	default:
		return nil
	}
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	result, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		if valkey.HasErrorPrefix(err, "redis: nil") {
			return nil, fmt.Errorf("%w: %w", store.ErrNotFound, err)
		}

		return nil, fmt.Errorf("can't fetch from valkey: %w", err)
	}

	return []byte(result), nil
}

func (s *Store) Set(ctx context.Context, key string, value []byte, expiry time.Duration) error {
	if _, err := s.rdb.Set(ctx, key, string(value), expiry).Result(); err != nil {
		return fmt.Errorf("can't set %q in valkey: %w", key, err)
	}

	return nil
}

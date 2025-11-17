package valkey

import (
	"context"
	"time"

	"github.com/TecharoHQ/anubis/lib/store"
	valkey "github.com/redis/go-redis/v9"
)

// Store implements store.Interface on top of Redis/Valkey.
type Store struct {
	client redisClient
}

var _ store.Interface = (*Store)(nil)

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	cmd := s.client.Get(ctx, key)
	if err := cmd.Err(); err != nil {
		if err == valkey.Nil {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	return cmd.Bytes()
}

func (s *Store) Set(ctx context.Context, key string, value []byte, expiry time.Duration) error {
	return s.client.Set(ctx, key, value, expiry).Err()
}

func (s *Store) Delete(ctx context.Context, key string) error {
	res := s.client.Del(ctx, key)
	if err := res.Err(); err != nil {
		return err
	}
	if n, _ := res.Result(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// IsPersistent tells Anubis this backend is “real” storage, not in-memory.
func (s *Store) IsPersistent() bool {
	return true
}

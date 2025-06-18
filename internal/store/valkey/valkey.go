package valkey

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/TecharoHQ/anubis/internal/store"
	valkey "github.com/redis/go-redis/v9"
)

var (
	_ store.Impl = &Store{}
)

type Store struct {
	rdb *valkey.Client
}

func New(rdb *valkey.Client) *Store {
	return &Store{rdb: rdb}
}

func (s *Store) Increment(ctx context.Context, segments []string) error {
	key := fmt.Sprintf("anubis:%s", strings.Join(segments, ":"))
	if err := s.rdb.Incr(ctx, key).Err(); err != nil {
		return err
	}

	return nil
}

func (s *Store) GetInt(ctx context.Context, segments []string) (int, error) {
	key := fmt.Sprintf("anubis:%s", strings.Join(segments, ":"))
	numStr, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return 0, err
	}

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, err
	}

	return num, nil
}

func (s *Store) MultiGetInt(ctx context.Context, segments [][]string) ([]int, error) {
	var keys []string
	for _, segment := range segments {
		key := fmt.Sprintf("anubis:%s", strings.Join(segment, ":"))
		keys = append(keys, key)
	}

	values, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	var errs []error

	result := make([]int, len(values))
	for i, val := range values {
		if val == nil {
			result[i] = 0
			errs = append(errs, fmt.Errorf("can't get key %s: value is null", keys[i]))
			continue
		}

		switch v := val.(type) {
		case string:
			num, err := strconv.Atoi(v)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't parse key %s: %w", keys[i], err))
				continue
			}

			result[i] = num
		default:
			errs = append(errs, fmt.Errorf("can't parse key %s: wanted type string but got type %T", keys[i], val))
		}
	}

	if len(errs) != 0 {
		return nil, fmt.Errorf("can't read from valkey: %w", errors.Join(errs...))
	}

	return result, nil
}

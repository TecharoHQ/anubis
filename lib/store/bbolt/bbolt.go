package bbolt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/TecharoHQ/anubis/lib/store"
	"go.etcd.io/bbolt"
)

// Sentinel error values used for testing and in admin-visible error messages.
var (
	ErrBucketDoesNotExist = errors.New("bbolt: bucket does not exist")
	ErrNotExists          = errors.New("bbolt: value does not exist in store")
)

// Store implements store.Interface backed by bbolt[1].
//
// In essence, bbolt is a hierarchical key/value store with a twist: every value
// needs to belong to a bucket. Buckets can contain an infinite number of
// buckets. As such, Anubis nests values in buckets. Each value in the store
// is given its own bucket with two keys:
//
// 1. data - The raw data, usually in JSON
// 2. expiry - The expiry time formatted as a time.RFC3339Nano timestamp string
//
// When Anubis stores a new bit of data, it creates a new bucket for that value.
// This allows the cleanup phase to iterate over every bucket in the database and
// only scan the expiry times without having to decode the entire record.
//
// bbolt is not suitable for environments where multiple instance of Anubis need
// to read from and write to the same backend store. For that, use the valkey
// storage backend.
//
// [1]: https://github.com/etcd-io/bbolt
type Store struct {
	bdb *bbolt.DB
}

// Delete a key from the datastore. If the key does not exist, return an error.
func (s *Store) Delete(ctx context.Context, key string) error {
	return s.bdb.Update(func(tx *bbolt.Tx) error {
		if tx.Bucket([]byte(key)) == nil {
			return fmt.Errorf("%w: %q", ErrNotExists, key)
		}

		return tx.DeleteBucket([]byte(key))
	})
}

// Get a value from the datastore.
//
// Because each value is stored in its own bucket with data and expiry keys,
// two get operations are required:
//
// 1. Get the expiry key, parse as time.RFC3339Nano. If the key has expired, run deletion in the background and return a "key not found" error.
// 2. Get the data key, copy into the result byteslice, return it.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	var result []byte

	if err := s.bdb.View(func(tx *bbolt.Tx) error {
		itemBucket := tx.Bucket([]byte(key))
		if itemBucket == nil {
			return fmt.Errorf("%w: %q", store.ErrNotFound, key)
		}

		expiryStr := itemBucket.Get([]byte("expiry"))
		if expiryStr == nil {
			return fmt.Errorf("[unexpected] %w: %q (expiry is nil)", store.ErrNotFound, key)
		}

		expiry, err := time.Parse(time.RFC3339Nano, string(expiryStr))
		if err != nil {
			return fmt.Errorf("[unexpected] %w: %w", store.ErrCantDecode, err)
		}

		if time.Now().After(expiry) {
			go s.Delete(context.Background(), key)
			return fmt.Errorf("%w: %q", store.ErrNotFound, key)
		}

		dataStr := itemBucket.Get([]byte("data"))
		if dataStr == nil {
			return fmt.Errorf("[unexpected] %w: %q (data is nil)", store.ErrNotFound, key)
		}

		result = make([]byte, len(dataStr))
		if n := copy(result, dataStr); n != len(dataStr) {
			return fmt.Errorf("[unexpected] %w: %d bytes copied of %d", store.ErrCantDecode, n, len(dataStr))
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// Set a value into the store with a given expiry.
func (s *Store) Set(ctx context.Context, key string, value []byte, expiry time.Duration) error {
	expires := time.Now().Add(expiry)

	return s.bdb.Update(func(tx *bbolt.Tx) error {
		valueBkt, err := tx.CreateBucketIfNotExists([]byte(key))
		if err != nil {
			return fmt.Errorf("%w: %w: %q (create bucket)", store.ErrCantEncode, err, key)
		}

		if err := valueBkt.Put([]byte("expiry"), []byte(expires.Format(time.RFC3339Nano))); err != nil {
			return fmt.Errorf("%w: %q (expiry)", store.ErrCantEncode, key)
		}

		if err := valueBkt.Put([]byte("data"), value); err != nil {
			return fmt.Errorf("%w: %q (data)", store.ErrCantEncode, key)
		}

		return nil
	})
}

func (s *Store) cleanup(ctx context.Context) error {
	now := time.Now()

	return s.bdb.Update(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(key []byte, valueBkt *bbolt.Bucket) error {
			var expiry time.Time
			var err error

			expiryStr := valueBkt.Get([]byte("expiry"))
			if expiryStr == nil {
				slog.Warn("while running cleanup, expiry is not set somehow, file a bug?", "key", string(key))
				return nil
			}

			expiry, err = time.Parse(time.RFC3339Nano, string(expiryStr))
			if err != nil {
				return fmt.Errorf("[unexpected] %w in bucket %q: %w", store.ErrCantDecode, string(key), err)
			}

			if now.After(expiry) {
				return valueBkt.DeleteBucket(key)
			}

			return nil
		})
	})
}

func (s *Store) cleanupThread(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.cleanup(ctx); err != nil {
				slog.Error("error during bbolt cleanup", "err", err)
			}
		}
	}
}

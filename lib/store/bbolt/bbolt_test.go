package bbolt

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/store/storetest"
	"go.etcd.io/bbolt"
)

func TestImpl(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	t.Log(path)
	data, err := json.Marshal(Config{
		Path: path,
	})
	if err != nil {
		t.Fatal(err)
	}

	storetest.Common(t, Factory{}, json.RawMessage(data))
}

// newTestStore returns a Store backed by a throwaway bbolt database that is
// closed when the test finishes.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	db, err := bbolt.Open(filepath.Join(t.TempDir(), "db"), 0600, nil)
	if err != nil {
		t.Fatalf("can't open bbolt database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return &Store{bdb: db}
}

// mustSet writes a value with the given relative expiry, failing the test on error.
func mustSet(t *testing.T, s *Store, key, value string, expiry time.Duration) {
	t.Helper()

	if err := s.Set(t.Context(), key, []byte(value), expiry); err != nil {
		t.Fatalf("Set(%q): %v", key, err)
	}
}

// readExpiry returns the expiry timestamp currently stored for key, as a Get
// would parse it. It fails the test if the bucket or expiry is missing.
func readExpiry(t *testing.T, s *Store, key string) time.Time {
	t.Helper()

	var out time.Time
	if err := s.bdb.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(key))
		if b == nil {
			t.Fatalf("bucket %q missing", key)
		}

		expiry, err := time.Parse(time.RFC3339Nano, string(b.Get([]byte("expiry"))))
		if err != nil {
			return err
		}
		out = expiry
		return nil
	}); err != nil {
		t.Fatalf("reading expiry for %q: %v", key, err)
	}

	return out
}

// rawData reads the raw data value for key directly, bypassing the expiry check
// in Get so tests can observe whether a bucket physically exists. It returns nil
// when the bucket is absent.
func rawData(t *testing.T, s *Store, key string) []byte {
	t.Helper()

	var out []byte
	if err := s.bdb.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(key))
		if b == nil {
			return nil
		}
		data := b.Get([]byte("data"))
		out = make([]byte, len(data))
		copy(out, data)
		return nil
	}); err != nil {
		t.Fatalf("reading data for %q: %v", key, err)
	}

	return out
}

// TestDeleteIfExpired guards against AWOO-015: a stale async delete scheduled by
// an expired Get must not erase a value that was refreshed (or otherwise differs
// from) the generation it observed.
func TestDeleteIfExpired(t *testing.T) {
	const key = "challenge"

	for _, tt := range []struct {
		setup       func(t *testing.T, s *Store) time.Time
		name        string
		wantValue   string
		wantPresent bool
	}{
		{
			name: "deletes the observed expired generation",
			setup: func(t *testing.T, s *Store) time.Time {
				mustSet(t, s, key, "old", -time.Minute)
				return readExpiry(t, s, key)
			},
			wantPresent: false,
		},
		{
			name: "preserves a refreshed generation",
			setup: func(t *testing.T, s *Store) time.Time {
				mustSet(t, s, key, "old", -time.Minute)
				observed := readExpiry(t, s, key)
				mustSet(t, s, key, "fresh", time.Hour)
				return observed
			},
			wantPresent: true,
			wantValue:   "fresh",
		},
		{
			name: "skips on generation mismatch",
			setup: func(t *testing.T, s *Store) time.Time {
				mustSet(t, s, key, "old", -time.Minute)
				// An expiry we never wrote: even though the stored value is
				// expired, it is a different generation and must be left alone.
				return time.Now().Add(-2 * time.Hour)
			},
			wantPresent: true,
			wantValue:   "old",
		},
		{
			name: "skips a non-expired observation",
			setup: func(t *testing.T, s *Store) time.Time {
				mustSet(t, s, key, "live", time.Hour)
				return readExpiry(t, s, key)
			},
			wantPresent: true,
			wantValue:   "live",
		},
		{
			name: "no-op when bucket is absent",
			setup: func(t *testing.T, s *Store) time.Time {
				return time.Now().Add(-time.Hour)
			},
			wantPresent: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			observed := tt.setup(t, s)

			if err := s.deleteIfExpired(t.Context(), key, observed); err != nil {
				t.Fatalf("deleteIfExpired(%q): %v", key, err)
			}

			got := rawData(t, s, key)
			switch {
			case tt.wantPresent && got == nil:
				t.Fatalf("key %q: want present with value %q, got deleted", key, tt.wantValue)
			case tt.wantPresent && string(got) != tt.wantValue:
				t.Errorf("key %q: want value %q, got %q", key, tt.wantValue, string(got))
			case !tt.wantPresent && got != nil:
				t.Errorf("key %q: want deleted, got value %q", key, string(got))
			}
		})
	}
}

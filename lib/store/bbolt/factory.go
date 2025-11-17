package bbolt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TecharoHQ/anubis/lib/store"
	"go.etcd.io/bbolt"
)

var (
	ErrMissingPath     = errors.New("bbolt: path is missing from config")
	ErrCantWriteToPath = errors.New("bbolt: can't write to path")
)

func init() {
	store.Register("bbolt", Factory{})
}

// Factory builds new instances of the bbolt storage backend according to
// configuration passed via a json.RawMessage.
type Factory struct{}

// Build parses and validates the bbolt storage backend Config and creates
// a new instance of it.
func (Factory) Build(ctx context.Context, data json.RawMessage) (store.Interface, error) {
	var config Config
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	if err := config.Valid(); err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrBadConfig, err)
	}

	bdb, err := bbolt.Open(config.Path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("can't open bbolt database %s: %w", config.Path, err)
	}

	result := &Store{
		bdb: bdb,
	}

	go result.cleanupThread(ctx)

	return result, nil
}

// Valid parses and validates the bbolt store Config or returns
// an error.
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

// Config is the bbolt storage backend configuration.
type Config struct {
	// Path is the filesystem path of the database. The folder must be writable to Anubis.
	Path string `json:"path"`
}

// Valid validates the configuration including checking if its containing folder is writable.
func (c Config) Valid() error {
	var errs []error

	if c.Path == "" {
		errs = append(errs, ErrMissingPath)
	} else {
		dir := filepath.Dir(c.Path)
		if err := os.WriteFile(filepath.Join(dir, ".test-file"), []byte(""), 0600); err != nil {
			errs = append(errs, ErrCantWriteToPath)
		}
		os.Remove(filepath.Join(dir, ".test-file"))
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

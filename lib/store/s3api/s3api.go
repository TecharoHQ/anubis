package s3api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/TecharoHQ/anubis/lib/store"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Store struct {
	s3     S3API
	bucket string
}

func (s *Store) Delete(ctx context.Context, key string) error {
	normKey := strings.ReplaceAll(key, ":", "/")
	// Emulate not found by probing first.
	if _, err := s.s3.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &normKey}); err != nil {
		return fmt.Errorf("%w: %w", store.ErrNotFound, err)
	}
	if _, err := s.s3.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &normKey}); err != nil {
		return fmt.Errorf("can't delete from s3: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	normKey := strings.ReplaceAll(key, ":", "/")
	out, err := s.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &normKey,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", store.ErrNotFound, err)
	}
	defer out.Body.Close()
	if msStr, ok := out.Metadata["x-anubis-expiry-ms"]; ok && msStr != "" {
		if ms, err := strconv.ParseInt(msStr, 10, 64); err == nil {
			if time.Now().UnixMilli() >= ms {
				_, _ = s.s3.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &normKey})
				return nil, store.ErrNotFound
			}
		}
	}
	b, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("can't read s3 object: %w", err)
	}
	return b, nil
}

func (s *Store) Set(ctx context.Context, key string, value []byte, expiry time.Duration) error {
	normKey := strings.ReplaceAll(key, ":", "/")
	// S3 has no native TTL; we store object with metadata X-Anubis-Expiry as epoch seconds.
	var meta map[string]string
	if expiry > 0 {
		exp := time.Now().Add(expiry).UnixMilli()
		meta = map[string]string{"x-anubis-expiry-ms": fmt.Sprintf("%d", exp)}
	}
	_, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   &s.bucket,
		Key:      &normKey,
		Body:     bytes.NewReader(value),
		Metadata: meta,
	})
	if err != nil {
		return fmt.Errorf("can't put s3 object: %w", err)
	}
	return nil
}

func (Store) IsPersistent() bool { return true }

package s3api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/store/storetest"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// mockS3 is an in-memory mock of the methods we use.
type mockS3 struct {
	data   map[string][]byte
	meta   map[string]map[string]string
	bucket string
	mu     sync.RWMutex
}

func (m *mockS3) PutObject(ctx context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	if m.meta == nil {
		m.meta = map[string]map[string]string{}
	}
	b, _ := io.ReadAll(in.Body)
	m.data[aws.ToString(in.Key)] = bytes.Clone(b)
	if in.Metadata != nil {
		m.meta[aws.ToString(in.Key)] = map[string]string{}
		for k, v := range in.Metadata {
			m.meta[aws.ToString(in.Key)][k] = v
		}
	}
	m.bucket = aws.ToString(in.Bucket)
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3) GetObject(ctx context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.data[aws.ToString(in.Key)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	out := &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(b))}
	if md, ok := m.meta[aws.ToString(in.Key)]; ok {
		out.Metadata = md
	}
	return out, nil
}

func (m *mockS3) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, aws.ToString(in.Key))
	delete(m.meta, aws.ToString(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3) HeadObject(ctx context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.data[aws.ToString(in.Key)]; !ok {
		return nil, fmt.Errorf("not found")
	}
	return &s3.HeadObjectOutput{}, nil
}

func TestImpl(t *testing.T) {
	mock := &mockS3{}
	f := Factory{Client: mock}

	data, _ := json.Marshal(Config{
		BucketName: "bucket",
	})

	storetest.Common(t, f, json.RawMessage(data))
}

func TestKeyNormalization(t *testing.T) {
	mock := &mockS3{}
	f := Factory{Client: mock}

	data, _ := json.Marshal(Config{
		BucketName: "anubis",
	})

	s, err := f.Build(t.Context(), json.RawMessage(data))
	if err != nil {
		t.Fatal(err)
	}

	key := "a:b:c"
	val := []byte("value")
	if err := s.Set(t.Context(), key, val, 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	// Ensure mock saw normalized key
	mock.mu.RLock()
	_, hasRaw := mock.data["a:b:c"]
	got, hasNorm := mock.data["a/b/c"]
	mock.mu.RUnlock()
	if hasRaw {
		t.Fatalf("mock contains raw key with colon; normalization failed")
	}
	if !hasNorm || !bytes.Equal(got, val) {
		t.Fatalf("normalized key missing or wrong value: got=%q", string(got))
	}

	// Get using colon key should work
	out, err := s.Get(t.Context(), key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(out, val) {
		t.Fatalf("Get returned wrong value: got=%q", string(out))
	}

	// Delete using colon key should delete normalized object
	if err := s.Delete(t.Context(), key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	// Give any async cleanup in tests a tick (not needed for mock, but harmless)
	time.Sleep(1 * time.Millisecond)
	mock.mu.RLock()
	_, exists := mock.data["a/b/c"]
	mock.mu.RUnlock()
	if exists {
		t.Fatalf("normalized key still exists after Delete")
	}
}

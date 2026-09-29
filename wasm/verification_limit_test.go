package wasm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestVerificationLimit(t *testing.T) {
	releases := make([]func(), 0, 4)
	for range 4 {
		release, err := acquireVerification(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	t.Cleanup(func() {
		for _, release := range releases {
			release()
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if release, err := acquireVerification(ctx); !errors.Is(err, context.DeadlineExceeded) {
		if release != nil {
			release()
		}
		t.Fatalf("fifth verification not bounded: %v", err)
	}
	releases[0]()
	releases = releases[1:]
	release, err := acquireVerification(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	releases = append(releases, release)
}

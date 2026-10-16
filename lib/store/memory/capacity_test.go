package memory

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/store"
)

func TestStoreCapacity(t *testing.T) {
	st := New(t.Context())
	for idx := range 10001 {
		if err := st.Set(t.Context(), fmt.Sprint(idx), []byte("value"), time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.Get(t.Context(), "0"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("oldest key retained beyond capacity: %v", err)
	}
	if _, err := st.Get(t.Context(), "10000"); err != nil {
		t.Fatal(err)
	}
}

func TestStoreCapacityPreservesUpdates(t *testing.T) {
	st := New(t.Context())
	for range 10001 {
		if err := st.Set(t.Context(), "same", []byte("value"), time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.Get(t.Context(), "same"); err != nil {
		t.Fatal(err)
	}
}

func TestCapacityWithConcurrentWrites(t *testing.T) {
	st := New(t.Context()).(*impl)
	var wg sync.WaitGroup
	for worker := range 16 {
		wg.Go(func() {
			for idx := range 1000 {
				if err := st.Set(t.Context(), fmt.Sprintf("%d-%d", worker, idx), []byte("value"), time.Hour); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
	if got := st.store.Len(); got > maxEntries {
		t.Fatalf("concurrent writes exceeded capacity: %d", got)
	}
}

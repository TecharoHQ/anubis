package decaymap

import (
	"testing"
	"time"
)

func TestImpl(t *testing.T) {
	dm := New[string, string]()

	dm.Set("test", "hi", 5*time.Minute)

	val, ok := dm.Get("test")
	if !ok {
		t.Error("somehow the test key was not set")
	}

	if val != "hi" {
		t.Errorf("wanted value %q, got: %q", "hi", val)
	}

	ok = dm.expire("test")
	if !ok {
		t.Error("somehow could not force-expire the test key")
	}

	_, ok = dm.Get("test")
	if ok {
		t.Error("got value even though it was supposed to be expired")
	}
}

func TestCleanup(t *testing.T) {
	dm := New[string, string]()

	dm.Set("test1", "hi1", 1*time.Second)
	dm.Set("test2", "hi2", 2*time.Second)
	dm.Set("test3", "hi3", 3*time.Second)

	dm.expire("test1") // Force expire test1
	dm.expire("test2") // Force expire test2

	dm.Cleanup()

	finalLen := dm.Len() // Get the length after cleanup

	if finalLen != 1 { // "test3" should be the only one left
		t.Errorf("Cleanup failed to remove expired entries. Expected length 1, got %d", finalLen)
	}

	if _, ok := dm.Get("test1"); ok { // Verify Get still behaves correctly after Cleanup
		t.Error("test1 should not be found after cleanup")
	}
	if _, ok := dm.Get("test2"); ok {
		t.Error("test2 should not be found after cleanup")
	}
	if val, ok := dm.Get("test3"); !ok || val != "hi3" {
		t.Error("test3 should still be found after cleanup")
	}
}

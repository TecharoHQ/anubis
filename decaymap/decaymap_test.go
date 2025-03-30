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

	time.Sleep(2 * time.Second)
	dm.Cleanup()

	if _, ok := dm.Get("test1"); ok {
		t.Error("test1 should have been cleaned up")
	}

	if _, ok := dm.Get("test2"); ok {
		t.Error("test2 should have been cleaned up")
	}

	if val, ok := dm.Get("test3"); !ok || val != "hi3" {
		t.Error("test3 should not have been cleaned up")
	}
}

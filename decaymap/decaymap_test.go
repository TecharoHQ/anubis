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

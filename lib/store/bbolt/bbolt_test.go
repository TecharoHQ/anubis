package bbolt

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis/lib/store/storetest"
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

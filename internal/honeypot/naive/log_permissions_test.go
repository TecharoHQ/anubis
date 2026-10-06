package naive

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

func TestIPLogPermissions(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "existing"}[existing], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ips.log")
			if existing {
				if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			i, err := New(&config.Honeypot{IPLogFile: path}, memory.New(ctx), slog.Default())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = i.fout.Close() }()
			stat, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if stat.Size() != 0 {
				t.Fatalf("existing contents remain: size=%d", stat.Size())
			}
			if got := stat.Mode().Perm(); got != 0600 {
				t.Fatalf("mode=%o", got)
			}
		})
	}
}

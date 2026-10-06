package naive

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

func TestInvalidClientIP(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	i, err := New(&config.Honeypot{}, memory.New(ctx), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"", "invalid", "1.2.3.999"} {
		t.Run(ip, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://example.com/", nil)
			r.Header.Set("X-Real-IP", ip)
			w := httptest.NewRecorder()
			i.ServeHTTP(w, r)
			if w.Code != http.StatusTeapot {
				t.Errorf("status=%d", w.Code)
			}
			if match, err := i.CheckNetwork().Check(r); err != nil || match {
				t.Fatalf("match=%v err=%v", match, err)
			}
		})
	}
}

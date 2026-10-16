package lib

import (
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis"
)

func TestCheckResponsesNoStore(t *testing.T) {
	for _, public := range []string{"", "https://anubis.example.com"} {
		t.Run(public, func(t *testing.T) {
			s := spawnAnubis(t, Options{PublicUrl: public})
			r := httptest.NewRequest("GET", anubis.APIPrefix+"check", nil)
			r.Header.Set("Accept-Encoding", "gzip")
			r.Header.Set("User-Agent", "Mozilla/5.0")
			r.Header.Set("X-Real-IP", "192.0.2.1")
			r.Header.Set("X-Forwarded-Host", "example.com")
			r.Header.Set("X-Forwarded-Proto", "https")
			r.Header.Set("X-Forwarded-Uri", "/test")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			want := 401
			if public != "" {
				want = 307
			}
			if w.Code != want {
				t.Fatalf("status=%d want %d", w.Code, want)
			}
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("status=%d cache=%q", w.Code, got)
			}
		})
	}
}

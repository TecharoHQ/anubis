package lib

import (
	"net/http/httptest"
	"testing"

	"github.com/TecharoHQ/anubis"
	"github.com/TecharoHQ/anubis/lib/config"
)

func TestPageCache(t *testing.T) {
	p := loadPolicies(t, "", 4)
	p.Impressum = &config.Impressum{Footer: "footer", Page: config.ImpressumPage{Title: "title", Body: "body"}}
	s := spawnAnubis(t, Options{Policy: p})
	for _, tc := range []struct {
		name  string
		serve func(*httptest.ResponseRecorder)
	}{
		{"benchmark", func(w *httptest.ResponseRecorder) { s.RenderBench(w, httptest.NewRequest("GET", "/", nil)) }},
		{"happy", func(w *httptest.ResponseRecorder) { s.ServeHTTPNext(w, httptest.NewRequest("GET", "/", nil)) }},
		{"imprint", func(w *httptest.ResponseRecorder) {
			s.mux.ServeHTTP(w, httptest.NewRequest("GET", anubis.APIPrefix+"imprint", nil))
		}},
		{"honeypot", func(w *httptest.ResponseRecorder) {
			r := httptest.NewRequest("GET", anubis.APIPrefix+"honeypot/test/init", nil)
			r.Header.Set("X-Real-IP", "192.0.2.1")
			s.mux.ServeHTTP(w, r)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tc.serve(w)
			if w.Code != 200 {
				t.Fatalf("status=%d", w.Code)
			}
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("cache=%q", got)
			}
		})
	}
}

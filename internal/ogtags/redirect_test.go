package ogtags

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TecharoHQ/anubis/lib/config"
	"github.com/TecharoHQ/anubis/lib/store/memory"
)

func TestOGRedirectOrigin(t *testing.T) {
	for _, sni := range []string{"", "auto"} {
		t.Run(sni, func(t *testing.T) {
			var reached atomic.Bool
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached.Store(true)
				w.Header().Set("Content-Type", "text/html")
				if _, err := w.Write([]byte(`<meta property="og:title" content="secret">`)); err != nil {
					t.Error(err)
				}
			}))
			defer other.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/external":
					http.Redirect(w, r, other.URL, http.StatusFound)
				case "/local":
					http.Redirect(w, r, "/document", http.StatusFound)
				default:
					w.Header().Set("Content-Type", "text/html")
					if _, err := w.Write([]byte(`<meta property="og:title" content="safe">`)); err != nil {
						t.Error(err)
					}
				}
			}))
			defer origin.Close()
			cache := NewOGTagCache(origin.URL, config.OpenGraph{Enabled: true, TimeToLive: time.Minute}, memory.New(t.Context()), TargetOptions{SNI: sni})
			for _, path := range []string{"/external", "/local"} {
				u, _ := url.Parse(path)
				tags, err := cache.GetOGTags(t.Context(), u, "example.com")
				if path == "/external" {
					if reached.Load() || tags["og:title"] == "secret" {
						t.Fatal("cross-origin redirect fetched protected metadata")
					}
				} else if err != nil || tags["og:title"] != "safe" {
					t.Fatalf("same-origin redirect: tags=%v err=%v", tags, err)
				}
			}
		})
	}
}

package lib

import (
	"net/http"
	"time"

	"github.com/TecharoHQ/anubis"
)

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     anubis.CookieName,
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
}

// https://github.com/oauth2-proxy/oauth2-proxy/blob/master/pkg/upstream/http.go#L124
type UnixRoundTripper struct {
	Transport *http.Transport
}

// set bare minimum stuff
func (t UnixRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if req.Host == "" {
		req.Host = "localhost"
	}
	req.URL.Host = req.Host // proxy error: no Host in request URL
	req.URL.Scheme = "http" // make http.Transport happy and avoid an infinite recursion
	return t.Transport.RoundTrip(req)
}

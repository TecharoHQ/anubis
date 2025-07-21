package entrypoint

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func newH2cServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()

	h2s := &http2.Server{}

	srv := httptest.NewServer(h2c.NewHandler(h, h2s))
	t.Cleanup(func() {
		srv.Close()
	})

	return srv
}

func TestH2CReverseProxy(t *testing.T) {
	h := &ackHandler{}

	srv := newH2cServer(t, h)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	rp := httptest.NewServer(newH2CReverseProxy(u))
	defer rp.Close()

	resp, err := rp.Client().Get(rp.URL)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("wrong status code from reverse proxy: %d", resp.StatusCode)
	}

	if !h.ack {
		t.Error("h2c handler was not executed")
	}
}

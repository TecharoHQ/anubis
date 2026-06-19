package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestMakeReverseProxy(t *testing.T) {
	type received struct {
		host  string
		path  string
		query string
		hdr   http.Header
	}

	for _, tt := range []struct {
		name       string
		targetHost string
		reqHost    string
		reqPath    string
		reqHeaders map[string]string
		wantHost   string // empty means "same as the target server's host"
	}{
		{
			name:    "default preserves inbound host",
			reqHost: "anubis.example.com",
			reqPath: "/foo/bar?baz=qux",
		},
		{
			name:       "target host override",
			targetHost: "upstream.internal",
			reqHost:    "anubis.example.com",
			reqPath:    "/",
			wantHost:   "upstream.internal",
		},
		{
			name:    "forwarding headers are passed through",
			reqHost: "anubis.example.com",
			reqPath: "/",
			reqHeaders: map[string]string{
				"X-Forwarded-For":   "203.0.113.7, 198.51.100.2",
				"X-Forwarded-Host":  "anubis.example.com",
				"X-Forwarded-Proto": "https",
				"Forwarded":         "for=203.0.113.7;proto=https",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotCh := make(chan received, 1)
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotCh <- received{
					host:  r.Host,
					path:  r.URL.Path,
					query: r.URL.RawQuery,
					hdr:   r.Header.Clone(),
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(target.Close)

			h, err := makeReverseProxy(target.URL, "", tt.targetHost, false, false)
			if err != nil {
				t.Fatalf("makeReverseProxy: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "http://"+tt.reqHost+tt.reqPath, nil)
			req.Host = tt.reqHost
			for k, v := range tt.reqHeaders {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("unexpected status from proxy: got %d, want %d", rec.Code, http.StatusNoContent)
			}

			got := <-gotCh

			wantHost := tt.wantHost
			if wantHost == "" {
				wantHost = tt.reqHost
			}
			if got.host != wantHost {
				t.Errorf("upstream Host: got %q, want %q", got.host, wantHost)
			}

			wantURL, _ := url.Parse("http://x" + tt.reqPath)
			if got.path != wantURL.Path {
				t.Errorf("upstream path: got %q, want %q", got.path, wantURL.Path)
			}
			if got.query != wantURL.RawQuery {
				t.Errorf("upstream query: got %q, want %q", got.query, wantURL.RawQuery)
			}

			for k, want := range tt.reqHeaders {
				if gotVal := got.hdr.Get(k); gotVal != want {
					t.Errorf("upstream header %q: got %q, want %q", k, gotVal, want)
				}
			}
		})
	}
}

package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lum8rjack/go-ja4h"
)

func TestJA4HReplacesClientFingerprint(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	r.Header.Add(JA4HHeaderName, "forged")
	r.Header.Add(JA4HHeaderName, "another forgery")
	want := ja4h.JA4H(r)
	JA4H(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := r.Header.Values(JA4HHeaderName)
		if len(values) != 1 || values[0] != want {
			t.Fatalf("fingerprint=%q want %q", values, want)
		}
	})).ServeHTTP(httptest.NewRecorder(), r)
}

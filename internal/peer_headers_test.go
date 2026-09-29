package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteIPOverridesForwardingHeaders(t *testing.T) {
	for _, secure := range []bool{false, true} {
		scheme := "http"
		if secure {
			scheme = "https"
		}
		req := httptest.NewRequest("GET", scheme+"://public.example/resource", nil)
		req.RemoteAddr = "192.0.2.42:1234"
		for _, key := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"} {
			req.Header.Set(key, "forged")
		}
		h := RemoteXRealIP(true, "tcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, want := range map[string]string{"Forwarded": "", "X-Forwarded-For": "192.0.2.42", "X-Forwarded-Host": "public.example", "X-Forwarded-Proto": scheme, "X-Real-IP": "192.0.2.42"} {
				if got := r.Header.Get(key); got != want {
					t.Errorf("%s=%q want %q", key, got, want)
				}
			}
		}))
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}

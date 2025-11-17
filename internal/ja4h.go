package internal

import (
	"net/http"

	"github.com/lum8rjack/go-ja4h"
)

func JA4H(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Add("X-Http-Fingerprint-JA4H", ja4h.JA4H(r))
		next.ServeHTTP(w, r)
	})
}

package internal

import (
	"net/http"

	"github.com/lum8rjack/go-ja4h"
)

// JA4HHeaderName is the name of the HTTP header that the [JA4H] middleware adds
// to requests, holding the request's JA4H fingerprint. It is also used to
// detect whether a policy references the fingerprint, so that the relatively
// expensive computation can be skipped when no rule needs it.
const JA4HHeaderName = "X-Http-Fingerprint-Ja4h"

func JA4H(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Add(JA4HHeaderName, ja4h.JA4H(r))
		next.ServeHTTP(w, r)
	})
}

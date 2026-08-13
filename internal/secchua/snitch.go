package secchua

import "net/http"

// Snitch makes clients snitch on all their Sec-Ch-Ua headers so Anubis can make a
// more informed decision.
func Snitch(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Accept-Ch", "Sec-CH-UA-Arch, Sec-CH-UA-Bitness, Sec-CH-UA-Platform-Version")
		next.ServeHTTP(w, r)
	})
}

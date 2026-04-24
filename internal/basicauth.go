package internal

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
)

// BasicAuth wraps next in HTTP Basic authentication using the provided
// credentials. If either username or password is empty, next is returned
// unchanged and a debug log line is emitted.
//
// Credentials are compared in constant time to avoid leaking information
// through timing side channels.
func BasicAuth(realm, username, password string, next http.Handler) http.Handler {
	if username == "" || password == "" {
		slog.Debug("skipping middleware, basic auth credentials are empty")
		return next
	}

	expectedUser := sha256.Sum256([]byte(username))
	expectedPass := sha256.Sum256([]byte(password))
	challenge := fmt.Sprintf("Basic realm=%q, charset=\"UTF-8\"", realm)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			unauthorized(w, challenge)
			return
		}

		gotUser := sha256.Sum256([]byte(user))
		gotPass := sha256.Sum256([]byte(pass))

		userMatch := subtle.ConstantTimeCompare(gotUser[:], expectedUser[:])
		passMatch := subtle.ConstantTimeCompare(gotPass[:], expectedPass[:])

		if userMatch&passMatch != 1 {
			unauthorized(w, challenge)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter, challenge string) {
	w.Header().Set("WWW-Authenticate", challenge)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}
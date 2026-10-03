package internal

import (
	"net/http"
	"time"
)

func NewHTTPServer(handler http.Handler) http.Server {
	return http.Server{
		Handler:           handler,
		ErrorLog:          GetFilteredHTTPLogger(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
}

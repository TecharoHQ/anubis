package internal

import (
	"log/slog"
	"net/http"
)

func GetRequestLogger(base *slog.Logger, r *http.Request) *slog.Logger {
	return base.With(
		"host", r.Host,
		"method", r.Method,
		"path", r.URL.Path,
		"user_agent", r.UserAgent(),
		"accept_language", r.Header.Get("Accept-Language"),
		"priority", r.Header.Get("Priority"),
		"x-forwarded-for",
		r.Header.Get("X-Forwarded-For"),
		"x-real-ip", r.Header.Get("X-Real-Ip"),
	)
}

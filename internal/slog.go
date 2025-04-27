package internal

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

func InitSlog(level string) {
	var programLevel slog.Level
	if err := (&programLevel).UnmarshalText([]byte(level)); err != nil {
		fmt.Fprintf(os.Stderr, "invalid log level %s: %v, using info\n", level, err)
		programLevel = slog.LevelInfo
	}

	leveler := &slog.LevelVar{}
	leveler.Set(programLevel)

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     leveler,
	})
	slog.SetDefault(slog.New(h))
}

func GetRequestLogger(r *http.Request) *slog.Logger {
	return slog.With(
		"user_agent", r.UserAgent(),
		"accept_language", r.Header.Get("Accept-Language"),
		"priority", r.Header.Get("Priority"),
		"x-forwarded-for",
		r.Header.Get("X-Forwarded-For"),
		"x-real-ip", r.Header.Get("X-Real-Ip"),
	)
}

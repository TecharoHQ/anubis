package internal

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
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

// ErrorLogFilter is used to suppress "context canceled" logs from the http server when a request is canceled (e.g., when a client disconnects).
type ErrorLogFilter struct {
	Unwrap *log.Logger
}

func (elf *ErrorLogFilter) Write(p []byte) (n int, err error) {
	logMessage := string(p)
	if strings.Contains(logMessage, "context canceled") {
		return len(p), nil // Suppress the log by doing nothing
	}
	if elf.Unwrap != nil {
		return elf.Unwrap.Writer().Write(p)
	}
	return len(p), nil
}

func GetFilteredHTTPLogger() *log.Logger {
	stdErrLogger := log.New(os.Stderr, "", log.LstdFlags) // essentially what the default logger is.
	return log.New(&ErrorLogFilter{Unwrap: stdErrLogger}, "", 0)
}

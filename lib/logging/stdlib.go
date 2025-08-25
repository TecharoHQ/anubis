package logging

import (
	"bytes"
	"context"
	"log"
	"log/slog"
	"time"
)

// handlerWriter is an io.Writer that calls a Handler.
// It is used to link the default log.Logger to the default slog.Logger.
//
// Adapted from https://cs.opensource.google/go/go/+/refs/tags/go1.24.5:src/log/slog/logger.go;l=62
type handlerWriter struct {
	h     slog.Handler
	level slog.Leveler
}

func (w *handlerWriter) Write(buf []byte) (int, error) {
	level := w.level.Level()
	if !w.h.Enabled(context.Background(), level) {
		return 0, nil
	}
	var pc uintptr

	// Remove final newline.
	origLen := len(buf) // Report that the entire buf was written.
	buf = bytes.TrimSuffix(buf, []byte{'\n'})
	r := slog.NewRecord(time.Now(), level, string(buf), pc)
	return origLen, w.h.Handle(context.Background(), r)
}

func StdlibLogger(next slog.Handler, level slog.Level) *log.Logger {
	return log.New(&handlerWriter{h: next, level: level}, "", log.LstdFlags)
}

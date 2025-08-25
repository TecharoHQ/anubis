package logging

import (
	"fmt"
	"log/slog"
	"os"
)

func Init(level string) slog.Handler {
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
	return h
}

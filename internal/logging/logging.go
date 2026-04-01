package logging

import (
	"log/slog"
	"os"
	"strings"
)

func New(level string) *slog.Logger {
	options := &slog.HandlerOptions{
		Level: parseLevel(level),
	}

	handler := slog.NewTextHandler(os.Stderr, options)
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

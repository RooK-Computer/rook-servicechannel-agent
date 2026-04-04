package logging

import (
	"context"
	"encoding/json"
	"fmt"
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

func DebugEnabled(logger *slog.Logger) bool {
	return logger != nil && logger.Enabled(context.Background(), slog.LevelDebug)
}

func JSONValue(value interface{}) string {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return JSONBytes(body)
}

func JSONBytes(body []byte) string {
	if len(body) == 0 {
		return "{}"
	}

	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		return string(body)
	}

	redactSecrets(value)

	redacted, err := json.Marshal(value)
	if err != nil {
		return string(body)
	}

	return string(redacted)
}

func redactSecrets(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			if shouldRedactKey(key) {
				typed[key] = "<redacted>"
				continue
			}
			redactSecrets(item)
		}
	case []interface{}:
		for _, item := range typed {
			redactSecrets(item)
		}
	}
}

func shouldRedactKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "password", "wifi-password", "wifi_password", "secret", "token":
		return true
	default:
		return false
	}
}

package app

import (
	"context"
	"log/slog"
	"testing"

	"rook-servicechannel-agent/internal/config"
)

func TestRunBootstrapReady(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: "https://backend.example.test",
		LogLevel:   "info",
	}, logger)

	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

package app

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"rook-servicechannel-agent/internal/config"
)

func TestRunWaitsForShutdown(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: "https://backend.example.test",
		LogLevel:   "info",
	}, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run(ctx)
	}()

	select {
	case err := <-errCh:
		t.Fatalf("Run returned early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error after shutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestRunPrintConfigReturnsImmediately(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL:  "https://backend.example.test",
		LogLevel:    "info",
		PrintConfig: true,
	}, logger)

	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

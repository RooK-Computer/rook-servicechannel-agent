package app

import (
	"context"
	"log/slog"

	"rook-servicechannel-agent/internal/config"
)

type App struct {
	config config.Config
	logger *slog.Logger
}

func New(cfg config.Config, logger *slog.Logger) App {
	return App{
		config: cfg,
		logger: logger,
	}
}

func (a App) Run(ctx context.Context) error {
	if a.config.PrintConfig {
		a.logger.Info("effective configuration", "summary", a.config.Summary())
		return nil
	}

	a.logger.Info("rook agent bootstrap ready",
		"backend_url", a.config.BackendURL,
		"console_id", emptyAsUnset(a.config.ConsoleID),
		"mode", "bootstrap",
	)

	select {
	case <-ctx.Done():
		a.logger.Info("shutdown requested", "reason", ctx.Err())
		return nil
	default:
		return nil
	}
}

func emptyAsUnset(value string) string {
	if value == "" {
		return "<unset>"
	}
	return value
}

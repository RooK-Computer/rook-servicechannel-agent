package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"rook-servicechannel-agent/internal/backend"
	"rook-servicechannel-agent/internal/config"
	"rook-servicechannel-agent/internal/sessionstate"
)

type App struct {
	config config.Config
	logger *slog.Logger
	stdout io.Writer
}

func New(cfg config.Config, logger *slog.Logger, stdout io.Writer) App {
	return App{
		config: cfg,
		logger: logger,
		stdout: stdout,
	}
}

func (a App) Run(ctx context.Context) error {
	switch a.config.Command {
	case config.ConfigCommand:
		fmt.Fprintln(a.stdout, a.config.Summary())
		return nil
	case config.StartCommand:
		return a.startSession(ctx)
	case config.StatusCommand:
		return a.printSessionStatus(ctx)
	case config.PinCommand:
		return a.printSessionPIN()
	case config.PingCommand:
		return a.sendHeartbeat(ctx)
	case config.StopCommand:
		return a.stopSession(ctx)
	}

	a.logger.Info("rook agent bootstrap ready",
		"backend_url", a.config.BackendURL,
		"console_id", emptyAsUnset(a.config.ConsoleID),
		"mode", "bootstrap",
	)

	<-ctx.Done()
	a.logger.Info("shutdown requested", "reason", ctx.Err())
	return nil
}

func emptyAsUnset(value string) string {
	if value == "" {
		return "<unset>"
	}
	return value
}

func (a App) startSession(ctx context.Context) error {
	client, err := backend.NewClient(a.config.BackendURL, nil)
	if err != nil {
		return err
	}

	response, err := client.BeginSession(ctx, backend.StartSupportSessionRequest{})
	if err != nil {
		return err
	}

	if err := sessionstate.New(a.config.StatePath).Save(sessionstate.State{Session: response.Session}); err != nil {
		return err
	}

	printSession(a.stdout, response.Session)
	return nil
}

func (a App) printSessionStatus(ctx context.Context) error {
	client, err := backend.NewClient(a.config.BackendURL, nil)
	if err != nil {
		return err
	}

	pin, err := a.resolvePIN()
	if err != nil {
		return err
	}

	response, err := client.GetSessionStatus(ctx, backend.SessionStatusRequest{PIN: pin})
	if err != nil {
		return err
	}

	if err := sessionstate.New(a.config.StatePath).Save(sessionstate.State{Session: response.Session}); err != nil {
		return err
	}

	printSession(a.stdout, response.Session)
	return nil
}

func (a App) printSessionPIN() error {
	pin, err := a.resolvePIN()
	if err != nil {
		return err
	}

	fmt.Fprintln(a.stdout, pin)
	return nil
}

func (a App) sendHeartbeat(ctx context.Context) error {
	client, err := backend.NewClient(a.config.BackendURL, nil)
	if err != nil {
		return err
	}

	pin, err := a.resolvePIN()
	if err != nil {
		return err
	}

	if _, err := client.SendSessionHeartbeat(ctx, backend.SessionHeartbeatRequest{PIN: pin}); err != nil {
		return err
	}

	fmt.Fprintln(a.stdout, "heartbeat sent")
	return nil
}

func (a App) stopSession(ctx context.Context) error {
	client, err := backend.NewClient(a.config.BackendURL, nil)
	if err != nil {
		return err
	}

	pin, err := a.resolvePIN()
	if err != nil {
		return err
	}

	if _, err := client.EndSession(ctx, backend.EndSupportSessionRequest{PIN: pin}); err != nil {
		return err
	}

	if err := sessionstate.New(a.config.StatePath).Clear(); err != nil {
		return err
	}

	fmt.Fprintln(a.stdout, "session ended")
	return nil
}

func (a App) resolvePIN() (string, error) {
	if a.config.SessionPIN != "" {
		return a.config.SessionPIN, nil
	}

	state, err := sessionstate.New(a.config.StatePath).Load()
	if err != nil {
		if errors.Is(err, sessionstate.ErrStateNotFound) {
			return "", errors.New("no active session state found; use start first or provide --pin")
		}
		return "", err
	}

	return state.Session.PIN, nil
}

func printSession(w io.Writer, session backend.SupportSession) {
	fmt.Fprintf(w, "status=%s\n", session.Status)
	fmt.Fprintf(w, "pin=%s\n", session.PIN)
	fmt.Fprintf(w, "ip_address=%s\n", session.IPAddress)
}

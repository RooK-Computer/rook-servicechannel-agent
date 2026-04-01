package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"rook-servicechannel-agent/internal/backend"
	"rook-servicechannel-agent/internal/config"
	"rook-servicechannel-agent/internal/sessionstate"
)

type App struct {
	config config.Config
	logger *slog.Logger
	stdin  io.Reader
	stdout io.Writer

	heartbeatInterval time.Duration
	heartbeatMu       sync.Mutex
	heartbeatCancel   context.CancelFunc
	heartbeatPIN      string
}

func New(cfg config.Config, logger *slog.Logger, stdin io.Reader, stdout io.Writer) App {
	return App{
		config: cfg,
		logger: logger,
		stdin:  stdin,
		stdout: stdout,

		heartbeatInterval: backend.HeartbeatFrequency,
	}
}

func (a App) Run(ctx context.Context) error {
	switch a.config.Command {
	case config.InteractiveCommand:
		return a.runInteractive(ctx)
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

func (a *App) runInteractive(ctx context.Context) error {
	fmt.Fprintln(a.stdout, "Entering interactive mode. Type 'help' for commands.")

	lines := make(chan string)
	scanErrors := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(a.stdin)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			scanErrors <- err
		}
		close(lines)
	}()

	for {
		fmt.Fprint(a.stdout, "rook> ")

		select {
		case <-ctx.Done():
			a.stopAutomaticHeartbeat()
			fmt.Fprintln(a.stdout, "\ninteractive mode stopped")
			return nil
		case err := <-scanErrors:
			a.stopAutomaticHeartbeat()
			return err
		case line, ok := <-lines:
			if !ok {
				a.stopAutomaticHeartbeat()
				fmt.Fprintln(a.stdout, "\ninteractive mode ended")
				return nil
			}

			command := strings.TrimSpace(line)
			if command == "" {
				continue
			}

			if err := a.handleInteractiveCommand(ctx, command); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				fmt.Fprintf(a.stdout, "error: %v\n", err)
			}
		}
	}
}

func (a *App) handleInteractiveCommand(ctx context.Context, command string) error {
	switch strings.ToLower(command) {
	case "help":
		fmt.Fprintln(a.stdout, "commands: help, config, start, status, pin, ping, stop, exit")
		return nil
	case "config":
		fmt.Fprintln(a.stdout, a.config.Summary())
		return nil
	case "start":
		session, err := a.beginSession(ctx)
		if err != nil {
			return err
		}
		printSession(a.stdout, session)
		a.startAutomaticHeartbeat(ctx, session.PIN)
		return nil
	case "status":
		session, err := a.fetchSessionStatus(ctx)
		if err != nil {
			return err
		}
		printSession(a.stdout, session)
		return nil
	case "pin":
		return a.printSessionPIN()
	case "ping":
		return a.sendHeartbeat(ctx)
	case "stop":
		return a.stopSession(ctx)
	case "exit", "quit":
		a.stopAutomaticHeartbeat()
		fmt.Fprintln(a.stdout, "interactive mode exited")
		return io.EOF
	default:
		return fmt.Errorf("unknown interactive command %q", command)
	}
}

func emptyAsUnset(value string) string {
	if value == "" {
		return "<unset>"
	}
	return value
}

func (a *App) startSession(ctx context.Context) error {
	session, err := a.beginSession(ctx)
	if err != nil {
		return err
	}

	printSession(a.stdout, session)
	return nil
}

func (a *App) beginSession(ctx context.Context) (backend.SupportSession, error) {
	client, err := backend.NewClient(a.config.BackendURL, nil)
	if err != nil {
		return backend.SupportSession{}, err
	}

	response, err := client.BeginSession(ctx, backend.StartSupportSessionRequest{})
	if err != nil {
		return backend.SupportSession{}, err
	}

	if err := sessionstate.New(a.config.StatePath).Save(sessionstate.State{Session: response.Session}); err != nil {
		return backend.SupportSession{}, err
	}

	return response.Session, nil
}

func (a *App) printSessionStatus(ctx context.Context) error {
	session, err := a.fetchSessionStatus(ctx)
	if err != nil {
		return err
	}

	printSession(a.stdout, session)
	return nil
}

func (a *App) fetchSessionStatus(ctx context.Context) (backend.SupportSession, error) {
	client, err := backend.NewClient(a.config.BackendURL, nil)
	if err != nil {
		return backend.SupportSession{}, err
	}

	pin, err := a.resolvePIN()
	if err != nil {
		return backend.SupportSession{}, err
	}

	response, err := client.GetSessionStatus(ctx, backend.SessionStatusRequest{PIN: pin})
	if err != nil {
		return backend.SupportSession{}, err
	}

	if err := sessionstate.New(a.config.StatePath).Save(sessionstate.State{Session: response.Session}); err != nil {
		return backend.SupportSession{}, err
	}

	return response.Session, nil
}

func (a *App) printSessionPIN() error {
	pin, err := a.resolvePIN()
	if err != nil {
		return err
	}

	fmt.Fprintln(a.stdout, pin)
	return nil
}

func (a *App) sendHeartbeat(ctx context.Context) error {
	pin, err := a.resolvePIN()
	if err != nil {
		return err
	}

	return a.sendHeartbeatForPIN(ctx, pin, true)
}

func (a *App) sendHeartbeatForPIN(ctx context.Context, pin string, announce bool) error {
	client, err := backend.NewClient(a.config.BackendURL, nil)
	if err != nil {
		return err
	}

	if _, err := client.SendSessionHeartbeat(ctx, backend.SessionHeartbeatRequest{PIN: pin}); err != nil {
		return err
	}

	if announce {
		fmt.Fprintln(a.stdout, "heartbeat sent")
	}
	return nil
}

func (a *App) stopSession(ctx context.Context) error {
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

	a.stopAutomaticHeartbeat()

	if err := sessionstate.New(a.config.StatePath).Clear(); err != nil {
		return err
	}

	fmt.Fprintln(a.stdout, "session ended")
	return nil
}

func (a *App) resolvePIN() (string, error) {
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

func (a *App) startAutomaticHeartbeat(parent context.Context, pin string) {
	a.stopAutomaticHeartbeat()

	ctx, cancel := context.WithCancel(parent)

	a.heartbeatMu.Lock()
	a.heartbeatCancel = cancel
	a.heartbeatPIN = pin
	a.heartbeatMu.Unlock()

	fmt.Fprintf(a.stdout, "automatic heartbeat started (%s)\n", a.heartbeatInterval)

	go func() {
		ticker := time.NewTicker(a.heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := a.sendHeartbeatForPIN(ctx, pin, false); err != nil {
					var requestErr *backend.RequestError
					if errors.As(err, &requestErr) {
						fmt.Fprintf(a.stdout, "automatic heartbeat stopped: %v\n", err)
						a.stopAutomaticHeartbeat()
						return
					}

					fmt.Fprintf(a.stdout, "automatic heartbeat error: %v\n", err)
				}
			}
		}
	}()
}

func (a *App) stopAutomaticHeartbeat() {
	a.heartbeatMu.Lock()
	cancel := a.heartbeatCancel
	hadHeartbeat := cancel != nil
	a.heartbeatCancel = nil
	a.heartbeatPIN = ""
	a.heartbeatMu.Unlock()

	if cancel != nil {
		cancel()
	}

	if hadHeartbeat {
		fmt.Fprintln(a.stdout, "automatic heartbeat stopped")
	}
}

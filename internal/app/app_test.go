package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"rook-servicechannel-agent/internal/config"
)

func TestRunWaitsForShutdown(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: "https://backend.example.test",
		LogLevel:   "info",
		StatePath:  filepath.Join(t.TempDir(), "session.json"),
	}, logger, strings.NewReader(""), &bytes.Buffer{})

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
	var output bytes.Buffer
	application := New(config.Config{
		BackendURL:  "https://backend.example.test",
		LogLevel:    "info",
		StatePath:   filepath.Join(t.TempDir(), "session.json"),
		Command:     config.ConfigCommand,
		PrintConfig: true,
	}, logger, strings.NewReader(""), &output)

	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(output.String(), "backend_url=https://backend.example.test") {
		t.Fatalf("output = %q, want backend summary", output.String())
	}
}

func TestRunStartStatusPinPingStopFlow(t *testing.T) {
	serverState := "open"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/console/1/beginsession":
			_, _ = w.Write([]byte(`{"session":{"status":"open","pin":"1234","ipAddress":"10.8.0.2"}}`))
		case "/api/console/1/status":
			_, _ = w.Write([]byte(`{"session":{"status":"` + serverState + `","pin":"1234","ipAddress":"10.8.0.2"}}`))
		case "/api/console/1/ping":
			serverState = "active"
			_, _ = w.Write([]byte(`{}`))
		case "/api/console/1/endsession":
			serverState = "closed"
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	statePath := filepath.Join(t.TempDir(), "session.json")
	logger := slog.New(slog.DiscardHandler)

	runCommand := func(command config.Command) string {
		t.Helper()
		var output bytes.Buffer
		application := New(config.Config{
			BackendURL: server.URL,
			LogLevel:   "info",
			StatePath:  statePath,
			Command:    command,
		}, logger, strings.NewReader(""), &output)

		if err := application.Run(context.Background()); err != nil {
			t.Fatalf("Run(%s) returned error: %v", command, err)
		}

		return output.String()
	}

	startOutput := runCommand(config.StartCommand)
	if !strings.Contains(startOutput, "pin=1234") {
		t.Fatalf("start output = %q, want pin", startOutput)
	}

	pinOutput := runCommand(config.PinCommand)
	if strings.TrimSpace(pinOutput) != "1234" {
		t.Fatalf("pin output = %q, want 1234", pinOutput)
	}

	statusOutput := runCommand(config.StatusCommand)
	if !strings.Contains(statusOutput, "status=open") {
		t.Fatalf("status output = %q, want open", statusOutput)
	}

	pingOutput := runCommand(config.PingCommand)
	if strings.TrimSpace(pingOutput) != "heartbeat sent" {
		t.Fatalf("ping output = %q, want heartbeat sent", pingOutput)
	}

	statusOutput = runCommand(config.StatusCommand)
	if !strings.Contains(statusOutput, "status=active") {
		t.Fatalf("status output = %q, want active", statusOutput)
	}

	stopOutput := runCommand(config.StopCommand)
	if strings.TrimSpace(stopOutput) != "session ended" {
		t.Fatalf("stop output = %q, want session ended", stopOutput)
	}
}

func TestRunStatusRequiresStateOrPIN(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: "https://backend.example.test",
		LogLevel:   "info",
		StatePath:  filepath.Join(t.TempDir(), "session.json"),
		Command:    config.StatusCommand,
	}, logger, strings.NewReader(""), &bytes.Buffer{})

	err := application.Run(context.Background())
	if err == nil {
		t.Fatal("Run returned nil error, want missing state error")
	}

	if !strings.Contains(err.Error(), "no active session state found") {
		t.Fatalf("error = %v, want missing state message", err)
	}
}

func TestRunInteractiveModeStartsAutomaticHeartbeat(t *testing.T) {
	var (
		mu        sync.Mutex
		pingCount int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/console/1/beginsession":
			_, _ = w.Write([]byte(`{"session":{"status":"open","pin":"1234","ipAddress":"10.8.0.2"}}`))
		case "/api/console/1/status":
			_, _ = w.Write([]byte(`{"session":{"status":"active","pin":"1234","ipAddress":"10.8.0.2"}}`))
		case "/api/console/1/ping":
			mu.Lock()
			pingCount++
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
		case "/api/console/1/endsession":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	inputReader, inputWriter := io.Pipe()
	defer inputReader.Close()

	go func() {
		defer inputWriter.Close()
		fmt.Fprintln(inputWriter, "start")
		time.Sleep(80 * time.Millisecond)
		fmt.Fprintln(inputWriter, "status")
		fmt.Fprintln(inputWriter, "stop")
		fmt.Fprintln(inputWriter, "exit")
	}()

	var output bytes.Buffer
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: server.URL,
		LogLevel:   "info",
		StatePath:  filepath.Join(t.TempDir(), "session.json"),
		Command:    config.InteractiveCommand,
	}, logger, inputReader, &output)
	application.heartbeatInterval = 20 * time.Millisecond

	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	mu.Lock()
	gotPings := pingCount
	mu.Unlock()

	if gotPings == 0 {
		t.Fatal("automatic heartbeat did not send any ping")
	}

	if !strings.Contains(output.String(), "automatic heartbeat started") {
		t.Fatalf("output = %q, want heartbeat start message", output.String())
	}
}

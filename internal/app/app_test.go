package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"rook-servicechannel-agent/internal/config"
	"rook-servicechannel-agent/internal/ipc"
	"rook-servicechannel-agent/internal/network"
)

type fakeRunner struct {
	outputs map[string]string
	errors  map[string]error
}

func (r fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	call := name + " " + strings.Join(args, " ")
	if err, ok := r.errors[call]; ok {
		return "", err
	}
	return r.outputs[call], nil
}

func attachFakeNetwork(application *App) {
	runner := fakeRunner{
		outputs: map[string]string{
			"systemctl stop rook-openvpn-client.service":                "",
			"nmcli connection delete rook-support-wifi":                 "",
			"nmcli --terse --fields NAME,TYPE connection show --active": "",
			"systemctl is-active rook-openvpn-client.service":           "inactive\n",
		},
		errors: map[string]error{
			"ip -o -4 addr show dev rookvpn": errors.New("Cannot find device"),
		},
	}
	application.wifiManager = network.NewWiFiManager(runner)
	application.vpnManager = network.NewVPNManager(runner)
	application.cleaner = network.NewCleaner(application.wifiManager, application.vpnManager)
}

func TestRunWaitsForShutdown(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: "https://backend.example.test",
		LogLevel:   "info",
		StatePath:  filepath.Join(t.TempDir(), "session.json"),
	}, logger, strings.NewReader(""), &bytes.Buffer{})
	attachFakeNetwork(&application)

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
		attachFakeNetwork(&application)

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

func TestRunWiFiStatusReportsForeignAndSupportConnections(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	runCommand := func(outputForActive string) string {
		t.Helper()
		var output bytes.Buffer
		application := New(config.Config{
			BackendURL: "https://backend.example.test",
			LogLevel:   "info",
			StatePath:  filepath.Join(t.TempDir(), "session.json"),
			Command:    config.WiFiStatusCommand,
		}, logger, strings.NewReader(""), &output)
		runner := fakeRunner{
			outputs: map[string]string{
				"nmcli --terse --fields NAME,TYPE connection show --active": outputForActive,
			},
		}
		application.wifiManager = network.NewWiFiManager(runner)

		if err := application.Run(context.Background()); err != nil {
			t.Fatalf("Run(wifistatus) returned error: %v", err)
		}

		return output.String()
	}

	foreignOutput := runCommand("HomeNetwork:wifi\n")
	if !strings.Contains(foreignOutput, "wifi_active=true") || !strings.Contains(foreignOutput, "support_wifi_active=false") || !strings.Contains(foreignOutput, "active_connection=HomeNetwork") {
		t.Fatalf("foreign output = %q, want active foreign wifi status", foreignOutput)
	}

	supportOutput := runCommand("rook-support-wifi:wifi\n")
	if !strings.Contains(supportOutput, "wifi_active=true") || !strings.Contains(supportOutput, "support_wifi_active=true") || !strings.Contains(supportOutput, "active_connection=rook-support-wifi") {
		t.Fatalf("support output = %q, want active support wifi status", supportOutput)
	}
}

func TestRunInteractiveModeRequiresRunningService(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: "https://backend.example.test",
		LogLevel:   "info",
		StatePath:  filepath.Join(t.TempDir(), "session.json"),
		SocketPath: filepath.Join(t.TempDir(), "missing.sock"),
		Command:    config.InteractiveCommand,
	}, logger, strings.NewReader("exit\n"), &output)

	err := application.Run(context.Background())
	if err == nil {
		t.Fatal("Run returned nil error, want missing service error")
	}

	if !strings.Contains(err.Error(), "interactive mode requires a running service") {
		t.Fatalf("error = %v, want missing service guidance", err)
	}
}

func TestRunInteractiveModeUsesRunningServiceIPC(t *testing.T) {
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

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")
	statePath := filepath.Join(tmpDir, "session.json")

	service := New(config.Config{
		BackendURL: server.URL,
		LogLevel:   "info",
		StatePath:  statePath,
		SocketPath: socketPath,
		Command:    config.ServiceCommand,
	}, slog.New(slog.DiscardHandler), strings.NewReader(""), &bytes.Buffer{})
	attachFakeNetwork(&service)
	service.runtimeManager.SetHeartbeatInterval(20 * time.Millisecond)

	serviceCtx, cancelService := context.WithCancel(context.Background())
	defer cancelService()

	serviceErrCh := make(chan error, 1)
	go func() {
		serviceErrCh <- service.Run(serviceCtx)
	}()

	conn := waitForSocket(t, socketPath)
	_ = conn.Close()

	inputReader, inputWriter := io.Pipe()
	defer inputReader.Close()

	go func() {
		defer inputWriter.Close()
		fmt.Fprintln(inputWriter, "start")
		time.Sleep(80 * time.Millisecond)
		fmt.Fprintln(inputWriter, "wifistatus")
		fmt.Fprintln(inputWriter, "vpnstatus")
		fmt.Fprintln(inputWriter, "ping")
		fmt.Fprintln(inputWriter, "status")
		fmt.Fprintln(inputWriter, "stop")
		fmt.Fprintln(inputWriter, "exit")
	}()

	var output bytes.Buffer
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: server.URL,
		LogLevel:   "info",
		StatePath:  statePath,
		SocketPath: socketPath,
		Command:    config.InteractiveCommand,
	}, logger, inputReader, &output)

	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	cancelService()
	select {
	case err := <-serviceErrCh:
		if err != nil {
			t.Fatalf("service Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("service did not stop after cancellation")
	}

	mu.Lock()
	gotPings := pingCount
	mu.Unlock()

	if gotPings == 0 {
		t.Fatal("service heartbeat path did not send any ping")
	}

	if !strings.Contains(output.String(), "Connected to service") {
		t.Fatalf("output = %q, want service connection message", output.String())
	}

	if !strings.Contains(output.String(), "event: pin_assigned=1234") {
		t.Fatalf("output = %q, want pin assigned event", output.String())
	}

	if !strings.Contains(output.String(), "heartbeat sent") {
		t.Fatalf("output = %q, want heartbeat command output", output.String())
	}

	if !strings.Contains(output.String(), "wifi_active=false") {
		t.Fatalf("output = %q, want wifi status output", output.String())
	}

	if !strings.Contains(output.String(), "state=disconnected") {
		t.Fatalf("output = %q, want vpn status output", output.String())
	}
}

func TestRunServiceModeResumesPersistedSession(t *testing.T) {
	var (
		mu              sync.Mutex
		pingCount       int
		endSessionCount int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/console/1/beginsession":
			_, _ = w.Write([]byte(`{"session":{"status":"open","pin":"1234","ipAddress":"10.8.0.2"}}`))
		case "/api/console/1/ping":
			mu.Lock()
			pingCount++
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
		case "/api/console/1/endsession":
			mu.Lock()
			endSessionCount++
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	statePath := filepath.Join(t.TempDir(), "session.json")
	startOutput := func() string {
		t.Helper()
		var output bytes.Buffer
		application := New(config.Config{
			BackendURL: server.URL,
			LogLevel:   "info",
			StatePath:  statePath,
			Command:    config.StartCommand,
		}, slog.New(slog.DiscardHandler), strings.NewReader(""), &output)
		attachFakeNetwork(&application)

		if err := application.Run(context.Background()); err != nil {
			t.Fatalf("Run(start) returned error: %v", err)
		}

		return output.String()
	}()

	if !strings.Contains(startOutput, "pin=1234") {
		t.Fatalf("start output = %q, want pin", startOutput)
	}

	var output bytes.Buffer
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: server.URL,
		LogLevel:   "info",
		StatePath:  statePath,
		Command:    config.ServiceCommand,
	}, logger, strings.NewReader(""), &output)
	attachFakeNetwork(&application)
	application.runtimeManager.SetHeartbeatInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run(ctx)
	}()

	time.Sleep(70 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run(service) returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run(service) did not return after shutdown")
	}

	mu.Lock()
	gotPings := pingCount
	gotEnds := endSessionCount
	mu.Unlock()

	if gotPings == 0 {
		t.Fatal("service mode did not send any heartbeat")
	}

	if gotEnds != 1 {
		t.Fatalf("endSessionCount = %d, want 1", gotEnds)
	}
}

func TestRunServiceModeStartsIPCServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected backend request: %s", r.URL.Path)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "agent.sock")

	var output bytes.Buffer
	logger := slog.New(slog.DiscardHandler)
	application := New(config.Config{
		BackendURL: server.URL,
		LogLevel:   "info",
		StatePath:  filepath.Join(tmpDir, "session.json"),
		SocketPath: socketPath,
		Command:    config.ServiceCommand,
	}, logger, strings.NewReader(""), &output)
	attachFakeNetwork(&application)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run(ctx)
	}()

	conn := waitForSocket(t, socketPath)
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(ipc.Request{ID: "1", Action: ipc.GetStatusAction}); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	var response ipc.Response
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Decode() returned error: %v", err)
	}

	if !response.Success {
		t.Fatalf("response = %#v, want success", response)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run(service) returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run(service) did not return after shutdown")
	}
}

func waitForSocket(t *testing.T, socketPath string) net.Conn {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			return conn
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("socket %s did not become available", socketPath)
	return nil
}

func TestRunNetworkCommands(t *testing.T) {
	runner := fakeRunner{
		outputs: map[string]string{
			"nmcli --terse --fields SSID dev wifi list --rescan yes":             "Cafe\nOffice\n",
			"nmcli connection delete rook-support-wifi":                          "",
			"nmcli dev wifi connect Cafe password secret name rook-support-wifi": "",
			"nmcli --terse --fields NAME,TYPE connection show --active":          "rook-support-wifi:wifi\n",
			"systemctl is-active rook-openvpn-client.service":                    "active\n",
			"ip -o -4 addr show dev rookvpn":                                     "7: rookvpn    inet 10.8.0.2/24 scope global rookvpn\n",
			"systemctl start rook-openvpn-client.service":                        "",
			"systemctl stop rook-openvpn-client.service":                         "",
		},
	}

	runCommand := func(command config.Command, cfg config.Config) string {
		t.Helper()
		var output bytes.Buffer
		application := New(cfg, slog.New(slog.DiscardHandler), strings.NewReader(""), &output)
		application.wifiManager = network.NewWiFiManager(runner)
		application.vpnManager = network.NewVPNManager(runner)
		application.cleaner = network.NewCleaner(application.wifiManager, application.vpnManager)
		if err := application.Run(context.Background()); err != nil {
			t.Fatalf("Run(%s) returned error: %v", command, err)
		}
		return output.String()
	}

	cfg := config.Config{BackendURL: "https://backend.example.test", LogLevel: "info", StatePath: filepath.Join(t.TempDir(), "session.json")}

	cfg.Command = config.ScanWiFiCommand
	if out := runCommand(cfg.Command, cfg); !strings.Contains(out, "Cafe") {
		t.Fatalf("scan output = %q, want Cafe", out)
	}

	cfg.Command = config.ConnectWiFiCommand
	cfg.WiFiSSID = "Cafe"
	cfg.WiFiPassword = "secret"
	if out := runCommand(cfg.Command, cfg); !strings.Contains(out, "wifi connected") {
		t.Fatalf("connect output = %q, want wifi connected", out)
	}

	cfg.Command = config.VPNStatusCommand
	if out := runCommand(cfg.Command, cfg); !strings.Contains(out, "state=connected") {
		t.Fatalf("vpn status output = %q, want connected", out)
	}
}

package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rook-servicechannel-agent/internal/network"
	agentruntime "rook-servicechannel-agent/internal/runtime"
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

func newFakeNetworkAdapters() (*network.WiFiManager, *network.VPNManager) {
	runner := fakeRunner{
		outputs: map[string]string{
			"nmcli --terse --fields SSID dev wifi list --rescan yes":                        "Cafe\nOffice\n",
			"nmcli connection delete rook-support-wifi":                                     "",
			"nmcli --terse --fields NAME,TYPE connection show --active":                     "HomeNetwork:802-11-wireless\n",
			"nmcli --terse --fields IP4.ADDRESS connection show --active rook-support-wifi": "10.0.0.5/24\n",
			"nmcli dev wifi connect Cafe password secret name rook-support-wifi":            "",
			"systemctl is-active rook-openvpn-client.service":                               "inactive\n",
		},
		errors: map[string]error{
			"ip -o -4 addr show dev rookvpn": errors.New("Cannot find device"),
		},
	}
	return network.NewWiFiManager(runner), network.NewVPNManager(runner)
}

func newConnectReadyNetworkAdapters() (*network.WiFiManager, *network.VPNManager) {
	runner := fakeRunner{
		outputs: map[string]string{
			"nmcli --terse --fields SSID dev wifi list --rescan yes":                        "Cafe\nOffice\n",
			"nmcli connection delete rook-support-wifi":                                     "",
			"nmcli --terse --fields NAME,TYPE connection show --active":                     "rook-support-wifi:802-11-wireless\n",
			"nmcli --terse --fields IP4.ADDRESS connection show --active rook-support-wifi": "10.0.0.5/24\n",
			"nmcli dev wifi connect Cafe password secret name rook-support-wifi":            "",
			"systemctl is-active rook-openvpn-client.service":                               "inactive\n",
		},
		errors: map[string]error{
			"ip -o -4 addr show dev rookvpn": errors.New("Cannot find device"),
		},
	}
	return network.NewWiFiManager(runner), network.NewVPNManager(runner)
}

func TestServerCreatesWorldWritableSocket(t *testing.T) {
	socketDir := filepath.Join(t.TempDir(), "runtime")
	socketPath := filepath.Join(socketDir, "agent.sock")
	manager := agentruntime.New("https://backend.example.test", filepath.Join(t.TempDir(), "session.json"))
	wifiManager, vpnManager := newFakeNetworkAdapters()
	ipcServer := NewServer(socketPath, slog.New(slog.DiscardHandler), manager, wifiManager, vpnManager)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- ipcServer.Run(ctx)
	}()

	conn := waitForSocket(t, socketPath)
	_ = conn.Close()

	socketInfo, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("Stat(socket) returned error: %v", err)
	}
	if got := socketInfo.Mode().Perm(); got != 0o666 {
		t.Fatalf("socket mode = %o, want 666", got)
	}

	dirInfo, err := os.Stat(socketDir)
	if err != nil {
		t.Fatalf("Stat(socket dir) returned error: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o755 {
		t.Fatalf("socket dir mode = %o, want 755", got)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
}

func TestServerStartSupportBroadcastsEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/console/1/beginsession":
			_, _ = w.Write([]byte(`{"session":{"status":"open","pin":"1234","ipAddress":"10.8.0.2"}}`))
		case "/api/console/1/ping":
			_, _ = w.Write([]byte(`{}`))
		case "/api/console/1/endsession":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	socketPath := filepath.Join(t.TempDir(), "agent.sock")
	manager := agentruntime.New(server.URL, filepath.Join(t.TempDir(), "session.json"))
	manager.SetHeartbeatInterval(20 * time.Millisecond)
	wifiManager, vpnManager := newFakeNetworkAdapters()
	ipcServer := NewServer(socketPath, slog.New(slog.DiscardHandler), manager, wifiManager, vpnManager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ipcServer.Run(ctx)
	}()

	conn := waitForSocket(t, socketPath)
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(Request{ID: "1", Action: StartSupportAction}); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	var response Response
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Decode(response) returned error: %v", err)
	}

	if !response.Success {
		t.Fatalf("response = %#v, want success", response)
	}

	var event Event
	if err := decoder.Decode(&event); err != nil {
		t.Fatalf("Decode(event 1) returned error: %v", err)
	}

	if event.Event != SupportStateChangedEvent {
		t.Fatalf("event = %q, want %q", event.Event, SupportStateChangedEvent)
	}

	if err := decoder.Decode(&event); err != nil {
		t.Fatalf("Decode(event 2) returned error: %v", err)
	}

	if event.Event != PinAssignedEvent {
		t.Fatalf("event = %q, want %q", event.Event, PinAssignedEvent)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
}

func TestServerReconnectCanReadCurrentStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/console/1/beginsession":
			_, _ = w.Write([]byte(`{"session":{"status":"open","pin":"1234","ipAddress":"10.8.0.2"}}`))
		case "/api/console/1/ping":
			_, _ = w.Write([]byte(`{}`))
		case "/api/console/1/endsession":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	socketPath := filepath.Join(t.TempDir(), "agent.sock")
	statePath := filepath.Join(t.TempDir(), "session.json")
	manager := agentruntime.New(server.URL, statePath)
	wifiManager, vpnManager := newFakeNetworkAdapters()
	ipcServer := NewServer(socketPath, slog.New(slog.DiscardHandler), manager, wifiManager, vpnManager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ipcServer.Run(ctx)
	}()

	conn := waitForSocket(t, socketPath)
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)
	if err := encoder.Encode(Request{ID: "1", Action: StartSupportAction}); err != nil {
		t.Fatalf("Encode(start) returned error: %v", err)
	}

	var discard Response
	if err := decoder.Decode(&discard); err != nil {
		t.Fatalf("Decode(start response) returned error: %v", err)
	}
	_ = conn.Close()

	conn = waitForSocket(t, socketPath)
	defer conn.Close()

	encoder = json.NewEncoder(conn)
	decoder = json.NewDecoder(conn)
	if err := encoder.Encode(Request{ID: "2", Action: GetStatusAction}); err != nil {
		t.Fatalf("Encode(status) returned error: %v", err)
	}

	var response Response
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Decode(status response) returned error: %v", err)
	}

	payloadBytes, err := json.Marshal(response.Payload)
	if err != nil {
		t.Fatalf("Marshal(payload) returned error: %v", err)
	}

	var payload StatusPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("Unmarshal(payload) returned error: %v", err)
	}

	if !payload.SupportActive {
		t.Fatal("SupportActive = false, want true")
	}

	if !payload.AnyWiFiActive {
		t.Fatal("AnyWiFiActive = false, want true")
	}

	if payload.SupportWiFiActive {
		t.Fatal("SupportWiFiActive = true, want false")
	}

	if payload.ActiveWiFiConnection != "HomeNetwork" {
		t.Fatalf("ActiveWiFiConnection = %q, want HomeNetwork", payload.ActiveWiFiConnection)
	}

	if payload.Session == nil || payload.Session.PIN != "1234" {
		t.Fatalf("payload.Session = %#v, want PIN 1234", payload.Session)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() returned error: %v", err)
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

func TestServerStopSupportReturnsInactiveStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/console/1/beginsession":
			_, _ = w.Write([]byte(`{"session":{"status":"open","pin":"1234","ipAddress":"10.8.0.2"}}`))
		case "/api/console/1/ping":
			_, _ = w.Write([]byte(`{}`))
		case "/api/console/1/endsession":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	socketPath := filepath.Join(t.TempDir(), "agent.sock")
	manager := agentruntime.New(server.URL, filepath.Join(t.TempDir(), "session.json"))
	wifiManager, vpnManager := newFakeNetworkAdapters()
	ipcServer := NewServer(socketPath, slog.New(slog.DiscardHandler), manager, wifiManager, vpnManager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ipcServer.Run(ctx)
	}()

	conn := waitForSocket(t, socketPath)
	defer conn.Close()
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(Request{ID: "1", Action: StartSupportAction}); err != nil {
		t.Fatalf("Encode(start) returned error: %v", err)
	}
	var response Response
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Decode(start response) returned error: %v", err)
	}
	for i := 0; i < 2; i++ {
		var event Event
		if err := decoder.Decode(&event); err != nil && err != io.EOF {
			t.Fatalf("Decode(start event) returned error: %v", err)
		}
	}

	if err := encoder.Encode(Request{ID: "2", Action: StopSupportAction}); err != nil {
		t.Fatalf("Encode(stop) returned error: %v", err)
	}
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Decode(stop response) returned error: %v", err)
	}

	payloadBytes, err := json.Marshal(response.Payload)
	if err != nil {
		t.Fatalf("Marshal(payload) returned error: %v", err)
	}

	var payload StatusPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("Unmarshal(payload) returned error: %v", err)
	}

	if payload.SupportActive {
		t.Fatal("SupportActive = true, want false")
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
}

func TestServerDebugLoggingShowsIPCMessagesAndRedactsSecrets(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "agent.sock")
	manager := agentruntime.New("https://backend.example.test", filepath.Join(t.TempDir(), "session.json"))
	wifiManager, vpnManager := newConnectReadyNetworkAdapters()

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logOutput, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ipcServer := NewServer(socketPath, logger, manager, wifiManager, vpnManager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ipcServer.Run(ctx)
	}()

	conn := waitForSocket(t, socketPath)
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(Request{
		ID:     "1",
		Action: ConnectWiFiAction,
		Payload: json.RawMessage(
			`{"ssid":"Cafe","password":"secret"}`,
		),
	}); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	var response Response
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Decode(response) returned error: %v", err)
	}
	if !response.Success {
		t.Fatalf("response = %#v, want success", response)
	}

	var event Event
	if err := decoder.Decode(&event); err != nil {
		t.Fatalf("Decode(event) returned error: %v", err)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	logs := logOutput.String()
	if !strings.Contains(logs, "ipc request received") || !strings.Contains(logs, "\\\"action\\\":\\\"ConnectWifi\\\"") {
		t.Fatalf("logs = %q, want ipc request log", logs)
	}
	if !strings.Contains(logs, "ipc outbound message") || !strings.Contains(logs, "\\\"success\\\":true") {
		t.Fatalf("logs = %q, want ipc response log", logs)
	}
	if strings.Contains(logs, "\\\"password\\\":\\\"secret\\\"") {
		t.Fatalf("logs = %q, password must be redacted", logs)
	}
	if !strings.Contains(logs, "\\\"password\\\":\\\"\\\\u003credacted\\\\u003e\\\"") {
		t.Fatalf("logs = %q, want redacted password", logs)
	}
}

func TestServerScanAndConnectWifi(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "agent.sock")
	manager := agentruntime.New("https://backend.example.test", filepath.Join(t.TempDir(), "session.json"))
	wifiManager, vpnManager := newConnectReadyNetworkAdapters()
	ipcServer := NewServer(socketPath, slog.New(slog.DiscardHandler), manager, wifiManager, vpnManager)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ipcServer.Run(ctx)
	}()

	conn := waitForSocket(t, socketPath)
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(Request{ID: "1", Action: ScanWiFiAction}); err != nil {
		t.Fatalf("Encode(scan) returned error: %v", err)
	}

	var response Response
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Decode(scan response) returned error: %v", err)
	}
	if !response.Success {
		t.Fatalf("scan response = %#v, want success", response)
	}

	var event Event
	if err := decoder.Decode(&event); err != nil {
		t.Fatalf("Decode(scan event) returned error: %v", err)
	}
	if event.Event != WiFiScanCompletedEvent {
		t.Fatalf("event = %q, want %q", event.Event, WiFiScanCompletedEvent)
	}

	payload, err := json.Marshal(ConnectWiFiPayload{SSID: "Cafe", Password: "secret"})
	if err != nil {
		t.Fatalf("Marshal(payload) returned error: %v", err)
	}

	if err := encoder.Encode(Request{ID: "2", Action: ConnectWiFiAction, Payload: payload}); err != nil {
		t.Fatalf("Encode(connect) returned error: %v", err)
	}
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("Decode(connect response) returned error: %v", err)
	}
	if !response.Success {
		t.Fatalf("connect response = %#v, want success", response)
	}
	if err := decoder.Decode(&event); err != nil {
		t.Fatalf("Decode(connect event) returned error: %v", err)
	}
	if event.Event != WiFiConnectionStateChangedEvent {
		t.Fatalf("event = %q, want %q", event.Event, WiFiConnectionStateChangedEvent)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
}

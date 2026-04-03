package ipc

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"rook-servicechannel-agent/internal/network"
	agentruntime "rook-servicechannel-agent/internal/runtime"
)

func TestClientRequestAndEventFlow(t *testing.T) {
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

	waitConn := waitForSocket(t, socketPath)
	_ = waitConn.Close()

	client, err := DialClient(socketPath)
	if err != nil {
		t.Fatalf("DialClient() returned error: %v", err)
	}
	defer client.Close()

	var status StatusPayload
	if err := client.Request(context.Background(), StartSupportAction, nil, &status); err != nil {
		t.Fatalf("Request(StartSupport) returned error: %v", err)
	}

	if !status.SupportActive {
		t.Fatal("SupportActive = false, want true")
	}

	event := waitEvent(t, client.Events())
	if event.Event != SupportStateChangedEvent {
		t.Fatalf("event.Event = %q, want %q", event.Event, SupportStateChangedEvent)
	}

	event = waitEvent(t, client.Events())
	if event.Event != PinAssignedEvent {
		t.Fatalf("event.Event = %q, want %q", event.Event, PinAssignedEvent)
	}

	var vpnStatus VPNStatusPayload
	if err := client.Request(context.Background(), VPNStatusAction, nil, &vpnStatus); err != nil {
		t.Fatalf("Request(VPNStatus) returned error: %v", err)
	}

	if vpnStatus.State != string(network.StateDisconnected) {
		t.Fatalf("vpnStatus.State = %q, want disconnected", vpnStatus.State)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after cancellation")
	}
}

func waitEvent(t *testing.T, events <-chan RawEvent) RawEvent {
	t.Helper()

	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("events channel closed before event arrived")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for IPC event")
		return RawEvent{}
	}
}

package runtime

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"rook-servicechannel-agent/internal/backend"
	"rook-servicechannel-agent/internal/sessionstate"
)

func TestManagerSessionLifecycle(t *testing.T) {
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
	manager := New(server.URL, statePath)

	session, err := manager.BeginSession(context.Background())
	if err != nil {
		t.Fatalf("BeginSession() returned error: %v", err)
	}

	if session.PIN != "1234" {
		t.Fatalf("PIN = %q, want 1234", session.PIN)
	}

	pin, err := manager.CurrentPIN("")
	if err != nil {
		t.Fatalf("CurrentPIN() returned error: %v", err)
	}

	if pin != "1234" {
		t.Fatalf("CurrentPIN() = %q, want 1234", pin)
	}

	status, err := manager.GetSessionStatus(context.Background(), "")
	if err != nil {
		t.Fatalf("GetSessionStatus() returned error: %v", err)
	}

	if status.Status != "open" {
		t.Fatalf("status = %q, want open", status.Status)
	}

	if err := manager.SendHeartbeat(context.Background(), ""); err != nil {
		t.Fatalf("SendHeartbeat() returned error: %v", err)
	}

	status, err = manager.GetSessionStatus(context.Background(), "")
	if err != nil {
		t.Fatalf("GetSessionStatus() after heartbeat returned error: %v", err)
	}

	if status.Status != "active" {
		t.Fatalf("status = %q, want active", status.Status)
	}

	if err := manager.StopSession(context.Background(), ""); err != nil {
		t.Fatalf("StopSession() returned error: %v", err)
	}

	_, err = sessionstate.New(statePath).Load()
	if !errors.Is(err, sessionstate.ErrStateNotFound) {
		t.Fatalf("Load() error = %v, want ErrStateNotFound", err)
	}
}

func TestManagerHeartbeatLoopSendsPings(t *testing.T) {
	var (
		mu        sync.Mutex
		pingCount int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/console/1/ping":
			mu.Lock()
			pingCount++
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	manager := New(server.URL, filepath.Join(t.TempDir(), "session.json"))
	manager.SetHeartbeatInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.StartHeartbeatLoop(ctx, "1234")
	time.Sleep(70 * time.Millisecond)
	manager.StopHeartbeatLoop()

	mu.Lock()
	got := pingCount
	mu.Unlock()

	if got == 0 {
		t.Fatal("heartbeat loop did not send any ping")
	}
}

func TestManagerRunServiceResumesSessionAndEndsItOnShutdown(t *testing.T) {
	var (
		mu              sync.Mutex
		pingCount       int
		endSessionCount int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
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
	if err := sessionstate.New(statePath).Save(sessionstate.State{
		Session: backend.SupportSession{
			Status:    backend.SupportSessionOpen,
			PIN:       "1234",
			IPAddress: "10.8.0.2",
		},
	}); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	manager := New(server.URL, statePath)
	manager.SetHeartbeatInterval(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		errCh <- manager.RunService(ctx)
	}()

	time.Sleep(70 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("RunService() returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunService() did not return after shutdown")
	}

	mu.Lock()
	gotPings := pingCount
	gotEnds := endSessionCount
	mu.Unlock()

	if gotPings == 0 {
		t.Fatal("RunService() did not resume heartbeats")
	}

	if gotEnds != 1 {
		t.Fatalf("endSessionCount = %d, want 1", gotEnds)
	}

	_, err := sessionstate.New(statePath).Load()
	if !errors.Is(err, sessionstate.ErrStateNotFound) {
		t.Fatalf("Load() error = %v, want ErrStateNotFound", err)
	}
}

func TestManagerRecoverAfterBootClearsStaleSession(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "session.json")
	if err := sessionstate.New(statePath).Save(sessionstate.State{
		Session: backend.SupportSession{
			Status:    backend.SupportSessionOpen,
			PIN:       "1234",
			IPAddress: "10.8.0.2",
		},
		BootID: "old-boot-id",
	}); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	manager := New("https://backend.example.test", statePath)
	manager.bootIDFunc = func() (string, error) {
		return "new-boot-id", nil
	}

	recovered, err := manager.RecoverAfterBoot()
	if err != nil {
		t.Fatalf("RecoverAfterBoot() returned error: %v", err)
	}

	if !recovered {
		t.Fatal("RecoverAfterBoot() = false, want true")
	}

	_, err = sessionstate.New(statePath).Load()
	if !errors.Is(err, sessionstate.ErrStateNotFound) {
		t.Fatalf("Load() error = %v, want ErrStateNotFound", err)
	}
}

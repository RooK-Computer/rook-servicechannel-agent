package sessionstate

import (
	"errors"
	"path/filepath"
	"testing"

	"rook-servicechannel-agent/internal/backend"
)

func TestStoreSaveAndLoad(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "session.json"))
	want := State{
		Session: backend.SupportSession{
			Status:    backend.SupportSessionOpen,
			PIN:       "1234",
			IPAddress: "10.8.0.2",
		},
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if got.Session.PIN != want.Session.PIN {
		t.Fatalf("PIN = %q, want %q", got.Session.PIN, want.Session.PIN)
	}
}

func TestStoreLoadMissingFile(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "missing.json"))

	_, err := store.Load()
	if !errors.Is(err, ErrStateNotFound) {
		t.Fatalf("Load() error = %v, want ErrStateNotFound", err)
	}
}

func TestStoreClearMissingFile(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "missing.json"))

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() returned error: %v", err)
	}
}

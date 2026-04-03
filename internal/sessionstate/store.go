package sessionstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"rook-servicechannel-agent/internal/backend"
)

var ErrStateNotFound = errors.New("session state not found")

type State struct {
	Session backend.SupportSession `json:"session"`
	BootID  string                 `json:"bootId,omitempty"`
}

func (s State) Validate() error {
	return s.Session.Validate()
}

type Store struct {
	path string
}

func New(path string) Store {
	return Store{path: path}
}

func (s Store) Save(state State) error {
	if err := state.Validate(); err != nil {
		return fmt.Errorf("validate state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.WriteFile(s.path, payload, 0o600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	return nil
}

func (s Store) Load() (State, error) {
	payload, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, ErrStateNotFound
		}
		return State{}, fmt.Errorf("read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(payload, &state); err != nil {
		return State{}, fmt.Errorf("decode state file: %w", err)
	}

	if err := state.Validate(); err != nil {
		return State{}, fmt.Errorf("validate state file: %w", err)
	}

	return state, nil
}

func (s Store) Clear() error {
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove state file: %w", err)
	}
	return nil
}

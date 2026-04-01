package config

import "testing"

func TestLoadUsesEnvironmentDefaults(t *testing.T) {
	cfg, err := Load(nil, []string{
		"ROOK_AGENT_BACKEND_URL=https://backend.example.test",
		"ROOK_AGENT_CONSOLE_ID=console-42",
		"ROOK_AGENT_LOG_LEVEL=debug",
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.BackendURL != "https://backend.example.test" {
		t.Fatalf("BackendURL = %q, want environment value", cfg.BackendURL)
	}

	if cfg.ConsoleID != "console-42" {
		t.Fatalf("ConsoleID = %q, want environment value", cfg.ConsoleID)
	}

	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want environment value", cfg.LogLevel)
	}
}

func TestLoadFlagsOverrideEnvironment(t *testing.T) {
	cfg, err := Load([]string{
		"--backend-url=https://flag.example.test",
		"--console-id=flag-console",
		"--log-level=warn",
	}, []string{
		"ROOK_AGENT_BACKEND_URL=https://env.example.test",
		"ROOK_AGENT_CONSOLE_ID=env-console",
		"ROOK_AGENT_LOG_LEVEL=debug",
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.BackendURL != "https://flag.example.test" {
		t.Fatalf("BackendURL = %q, want flag value", cfg.BackendURL)
	}

	if cfg.ConsoleID != "flag-console" {
		t.Fatalf("ConsoleID = %q, want flag value", cfg.ConsoleID)
	}

	if cfg.LogLevel != "warn" {
		t.Fatalf("LogLevel = %q, want flag value", cfg.LogLevel)
	}
}

func TestLoadRejectsEmptyBackendURL(t *testing.T) {
	_, err := Load([]string{"--backend-url="}, nil)
	if err == nil {
		t.Fatal("Load returned nil error, want validation error")
	}
}

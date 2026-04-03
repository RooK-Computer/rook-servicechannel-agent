package config

import "testing"

func TestLoadUsesEnvironmentDefaults(t *testing.T) {
	cfg, err := Load(nil, []string{
		"ROOK_AGENT_BACKEND_URL=https://backend.example.test",
		"ROOK_AGENT_CONSOLE_ID=console-42",
		"ROOK_AGENT_LOG_LEVEL=debug",
		"ROOK_AGENT_STATE_PATH=/tmp/rook-agent/session.json",
		"ROOK_AGENT_SOCKET_PATH=/tmp/rook-agent/agent.sock",
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

	if cfg.StatePath != "/tmp/rook-agent/session.json" {
		t.Fatalf("StatePath = %q, want environment value", cfg.StatePath)
	}

	if cfg.SocketPath != "/tmp/rook-agent/agent.sock" {
		t.Fatalf("SocketPath = %q, want environment value", cfg.SocketPath)
	}
}

func TestLoadFlagsOverrideEnvironment(t *testing.T) {
	cfg, err := Load([]string{
		"status",
		"--backend-url=https://flag.example.test",
		"--console-id=flag-console",
		"--log-level=warn",
		"--state-path=/tmp/rook-agent/state.json",
		"--socket-path=/tmp/rook-agent/agent.sock",
		"--ssid=TestNet",
		"--wifi-password=secret",
		"--pin=1234",
	}, []string{
		"ROOK_AGENT_BACKEND_URL=https://env.example.test",
		"ROOK_AGENT_CONSOLE_ID=env-console",
		"ROOK_AGENT_LOG_LEVEL=debug",
		"ROOK_AGENT_STATE_PATH=/tmp/rook-agent/session.json",
		"ROOK_AGENT_SOCKET_PATH=/tmp/rook-agent/from-env.sock",
		"ROOK_AGENT_WIFI_SSID=EnvNet",
		"ROOK_AGENT_WIFI_PASSWORD=envsecret",
		"ROOK_AGENT_PIN=9999",
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

	if cfg.StatePath != "/tmp/rook-agent/state.json" {
		t.Fatalf("StatePath = %q, want flag value", cfg.StatePath)
	}

	if cfg.SocketPath != "/tmp/rook-agent/agent.sock" {
		t.Fatalf("SocketPath = %q, want flag value", cfg.SocketPath)
	}

	if cfg.SessionPIN != "1234" {
		t.Fatalf("SessionPIN = %q, want flag value", cfg.SessionPIN)
	}

	if cfg.WiFiSSID != "TestNet" {
		t.Fatalf("WiFiSSID = %q, want flag value", cfg.WiFiSSID)
	}

	if cfg.WiFiPassword != "secret" {
		t.Fatalf("WiFiPassword = %q, want flag value", cfg.WiFiPassword)
	}

	if cfg.Command != StatusCommand {
		t.Fatalf("Command = %q, want %q", cfg.Command, StatusCommand)
	}
}

func TestLoadRejectsEmptyBackendURL(t *testing.T) {
	_, err := Load([]string{"--backend-url="}, nil)
	if err == nil {
		t.Fatal("Load returned nil error, want validation error")
	}
}

func TestLoadMapsPrintConfigToConfigCommand(t *testing.T) {
	cfg, err := Load([]string{"--print-config"}, []string{"ROOK_AGENT_STATE_PATH=/tmp/state.json"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Command != ConfigCommand {
		t.Fatalf("Command = %q, want %q", cfg.Command, ConfigCommand)
	}
}

func TestLoadMapsInteractiveFlagToInteractiveCommand(t *testing.T) {
	cfg, err := Load([]string{"--interactive"}, []string{"ROOK_AGENT_STATE_PATH=/tmp/state.json"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Command != InteractiveCommand {
		t.Fatalf("Command = %q, want %q", cfg.Command, InteractiveCommand)
	}
}

func TestLoadRejectsInteractiveWithExplicitCommand(t *testing.T) {
	_, err := Load([]string{"status", "--interactive"}, []string{"ROOK_AGENT_STATE_PATH=/tmp/state.json"})
	if err == nil {
		t.Fatal("Load returned nil error for conflicting interactive command")
	}
}

func TestLoadRejectsUnknownCommand(t *testing.T) {
	_, err := Load([]string{"unknown"}, []string{"ROOK_AGENT_STATE_PATH=/tmp/state.json"})
	if err == nil {
		t.Fatal("Load returned nil error for unknown command")
	}
}

func TestLoadRecognizesServiceCommand(t *testing.T) {
	cfg, err := Load([]string{"service"}, []string{"ROOK_AGENT_STATE_PATH=/tmp/state.json"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Command != ServiceCommand {
		t.Fatalf("Command = %q, want %q", cfg.Command, ServiceCommand)
	}
}

func TestLoadRecognizesNetworkCommands(t *testing.T) {
	commands := []Command{
		ScanWiFiCommand,
		WiFiStatusCommand,
		ConnectWiFiCommand,
		DisconnectWiFiCommand,
		VPNStatusCommand,
		VPNStartCommand,
		VPNStopCommand,
		CleanupCommand,
	}

	for _, command := range commands {
		cfg, err := Load([]string{string(command)}, []string{"ROOK_AGENT_STATE_PATH=/tmp/state.json"})
		if err != nil {
			t.Fatalf("Load(%s) returned error: %v", command, err)
		}

		if cfg.Command != command {
			t.Fatalf("Command = %q, want %q", cfg.Command, command)
		}
	}
}

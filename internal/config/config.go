package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultBackendURL = "http://localhost:8080"
	defaultLogLevel   = "info"
)

type Config struct {
	BackendURL   string
	ConsoleID    string
	LogLevel     string
	PrintConfig  bool
	Interactive  bool
	StatePath    string
	SocketPath   string
	SessionPIN   string
	WiFiSSID     string
	WiFiPassword string
	Command      Command
}

type Command string

const (
	RunCommand            Command = "run"
	ServiceCommand        Command = "service"
	InteractiveCommand    Command = "interactive"
	ConfigCommand         Command = "config"
	StartCommand          Command = "start"
	StatusCommand         Command = "status"
	PinCommand            Command = "pin"
	PingCommand           Command = "ping"
	StopCommand           Command = "stop"
	ScanWiFiCommand       Command = "scanwifi"
	WiFiStatusCommand     Command = "wifistatus"
	ConnectWiFiCommand    Command = "connectwifi"
	DisconnectWiFiCommand Command = "disconnectwifi"
	VPNStatusCommand      Command = "vpnstatus"
	VPNStartCommand       Command = "vpnstart"
	VPNStopCommand        Command = "vpnstop"
	CleanupCommand        Command = "cleanup"
)

func Load(args []string, env []string) (Config, error) {
	envMap := environmentMap(env)
	command, commandArgs, err := parseCommand(args)
	if err != nil {
		return Config{}, err
	}

	fs := flag.NewFlagSet("rook-agent", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := Config{
		Command: command,
	}
	fs.StringVar(&cfg.BackendURL, "backend-url", envOrDefault(envMap, "ROOK_AGENT_BACKEND_URL", defaultBackendURL), "Base URL for the RooK backend API")
	fs.StringVar(&cfg.ConsoleID, "console-id", envOrDefault(envMap, "ROOK_AGENT_CONSOLE_ID", ""), "Stable console identity used for backend communication")
	fs.StringVar(&cfg.LogLevel, "log-level", envOrDefault(envMap, "ROOK_AGENT_LOG_LEVEL", defaultLogLevel), "Log level (debug, info, warn, error)")
	fs.StringVar(&cfg.StatePath, "state-path", envOrDefault(envMap, "ROOK_AGENT_STATE_PATH", defaultStatePath()), "Path to the local session state file")
	fs.StringVar(&cfg.SocketPath, "socket-path", envOrDefault(envMap, "ROOK_AGENT_SOCKET_PATH", defaultSocketPath()), "Path to the local IPC Unix socket")
	fs.StringVar(&cfg.SessionPIN, "pin", envOrDefault(envMap, "ROOK_AGENT_PIN", ""), "Override the active session PIN for session-scoped commands")
	fs.StringVar(&cfg.WiFiSSID, "ssid", envOrDefault(envMap, "ROOK_AGENT_WIFI_SSID", ""), "SSID for WiFi connection commands")
	fs.StringVar(&cfg.WiFiPassword, "wifi-password", envOrDefault(envMap, "ROOK_AGENT_WIFI_PASSWORD", ""), "Password for WiFi connection commands")
	fs.BoolVar(&cfg.PrintConfig, "print-config", false, "Print the effective configuration and exit")
	fs.BoolVar(&cfg.Interactive, "interactive", false, "Run the agent in interactive prompt mode")

	if err := fs.Parse(commandArgs); err != nil {
		return Config{}, err
	}

	if cfg.PrintConfig && cfg.Command == RunCommand {
		cfg.Command = ConfigCommand
	}

	if cfg.Interactive {
		if cfg.Command != RunCommand && cfg.Command != InteractiveCommand {
			return Config{}, errors.New("interactive mode cannot be combined with another explicit command")
		}
		cfg.Command = InteractiveCommand
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.BackendURL) == "" {
		errs = append(errs, errors.New("backend URL must not be empty"))
	}

	if strings.TrimSpace(c.LogLevel) == "" {
		errs = append(errs, errors.New("log level must not be empty"))
	}

	if strings.TrimSpace(c.StatePath) == "" {
		errs = append(errs, errors.New("state path must not be empty"))
	}

	if strings.TrimSpace(c.SocketPath) == "" {
		errs = append(errs, errors.New("socket path must not be empty"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (c Config) Summary() string {
	consoleID := c.ConsoleID
	if consoleID == "" {
		consoleID = "<unset>"
	}

	return fmt.Sprintf("backend_url=%s console_id=%s log_level=%s state_path=%s socket_path=%s command=%s", c.BackendURL, consoleID, c.LogLevel, c.StatePath, c.SocketPath, c.Command)
}

func environmentMap(env []string) map[string]string {
	values := make(map[string]string, len(env))
	for _, item := range env {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[parts[0]] = parts[1]
	}
	return values
}

func envOrDefault(env map[string]string, key, fallback string) string {
	if value, ok := env[key]; ok && strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func parseCommand(args []string) (Command, []string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return RunCommand, args, nil
	}

	switch Command(args[0]) {
	case ServiceCommand, InteractiveCommand, ConfigCommand, StartCommand, StatusCommand, PinCommand, PingCommand, StopCommand, ScanWiFiCommand, WiFiStatusCommand, ConnectWiFiCommand, DisconnectWiFiCommand, VPNStatusCommand, VPNStartCommand, VPNStopCommand, CleanupCommand:
		return Command(args[0]), args[1:], nil
	default:
		return "", nil, fmt.Errorf("unknown command %q", args[0])
	}
}

func defaultStatePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		return filepath.Join(".rook-agent", "session.json")
	}

	return filepath.Join(configDir, "rook-agent", "session.json")
}

func defaultSocketPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		return filepath.Join(".rook-agent", "agent.sock")
	}

	return filepath.Join(configDir, "rook-agent", "agent.sock")
}

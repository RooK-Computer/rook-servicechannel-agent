package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	defaultBackendURL = "http://localhost:8080"
	defaultLogLevel   = "info"
)

type Config struct {
	BackendURL  string
	ConsoleID   string
	LogLevel    string
	PrintConfig bool
}

func Load(args []string, env []string) (Config, error) {
	envMap := environmentMap(env)

	fs := flag.NewFlagSet("rook-agent", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := Config{}
	fs.StringVar(&cfg.BackendURL, "backend-url", envOrDefault(envMap, "ROOK_AGENT_BACKEND_URL", defaultBackendURL), "Base URL for the RooK backend API")
	fs.StringVar(&cfg.ConsoleID, "console-id", envOrDefault(envMap, "ROOK_AGENT_CONSOLE_ID", ""), "Stable console identity used for backend communication")
	fs.StringVar(&cfg.LogLevel, "log-level", envOrDefault(envMap, "ROOK_AGENT_LOG_LEVEL", defaultLogLevel), "Log level (debug, info, warn, error)")
	fs.BoolVar(&cfg.PrintConfig, "print-config", false, "Print the effective configuration and exit")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
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

	return fmt.Sprintf("backend_url=%s console_id=%s log_level=%s", c.BackendURL, consoleID, c.LogLevel)
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

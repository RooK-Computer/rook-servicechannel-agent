package network

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const (
	SupportConnectionName = "rook-support-wifi"
	OpenVPNServiceName    = "rook-openvpn-client.service"
	OpenVPNInterfaceName  = "rookvpn"
	OpenVPNStatusFilePath = "/var/log/rook-openvpn/client-status.log"
)

type ConnectionState string

const (
	StateConnected    ConnectionState = "connected"
	StateDisconnected ConnectionState = "disconnected"
)

type WiFiNetwork struct {
	SSID string
}

type WiFiStatus struct {
	State                ConnectionState
	AnyActive            bool
	SupportActive        bool
	ActiveConnectionName string
}

type VPNStatus struct {
	State             ConnectionState
	ServiceActive     bool
	InterfacePresent  bool
	IPAddress         string
	StatusFilePresent bool
}

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

type WiFiManager struct {
	runner Runner
}

func NewWiFiManager(runner Runner) *WiFiManager {
	if runner == nil {
		runner = ExecRunner{}
	}

	return &WiFiManager{runner: runner}
}

func (m *WiFiManager) Scan(ctx context.Context) ([]WiFiNetwork, error) {
	output, err := m.runner.Run(ctx, "nmcli", "--terse", "--fields", "SSID", "dev", "wifi", "list", "--rescan", "yes")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	networks := make([]WiFiNetwork, 0)
	for _, line := range strings.Split(output, "\n") {
		ssid := strings.TrimSpace(strings.ReplaceAll(line, `\:`, `:`))
		if ssid == "" {
			continue
		}
		if _, ok := seen[ssid]; ok {
			continue
		}
		seen[ssid] = struct{}{}
		networks = append(networks, WiFiNetwork{SSID: ssid})
	}

	sort.Slice(networks, func(i, j int) bool {
		return networks[i].SSID < networks[j].SSID
	})

	return networks, nil
}

func (m *WiFiManager) Connect(ctx context.Context, ssid, password string) error {
	if strings.TrimSpace(ssid) == "" {
		return errors.New("ssid must not be empty")
	}
	if strings.TrimSpace(password) == "" {
		return errors.New("password must not be empty")
	}

	if err := m.Disconnect(ctx); err != nil {
		return err
	}

	_, err := m.runner.Run(ctx, "nmcli", "dev", "wifi", "connect", ssid, "password", password, "name", SupportConnectionName)
	return err
}

func (m *WiFiManager) Disconnect(ctx context.Context) error {
	_, err := m.runner.Run(ctx, "nmcli", "connection", "delete", SupportConnectionName)
	if err != nil {
		if isMissingConnection(err) {
			return nil
		}
		return err
	}
	return nil
}

func (m *WiFiManager) Status(ctx context.Context) (WiFiStatus, error) {
	output, err := m.runner.Run(ctx, "nmcli", "--terse", "--fields", "NAME,TYPE", "connection", "show", "--active")
	if err != nil {
		return WiFiStatus{}, err
	}

	status := WiFiStatus{State: StateDisconnected}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, connectionType, ok := parseNMCLIConnection(line)
		if !ok || !isWiFiConnectionType(connectionType) {
			continue
		}

		status.AnyActive = true
		if status.ActiveConnectionName == "" {
			status.ActiveConnectionName = name
		}

		if name == SupportConnectionName {
			status.SupportActive = true
			status.State = StateConnected
		}
	}

	return status, nil
}

type VPNManager struct {
	runner   Runner
	readFile func(string) ([]byte, error)
}

func NewVPNManager(runner Runner) *VPNManager {
	if runner == nil {
		runner = ExecRunner{}
	}

	return &VPNManager{
		runner:   runner,
		readFile: os.ReadFile,
	}
}

func (m *VPNManager) Start(ctx context.Context) error {
	_, err := m.runner.Run(ctx, "systemctl", "start", OpenVPNServiceName)
	return err
}

func (m *VPNManager) Stop(ctx context.Context) error {
	_, err := m.runner.Run(ctx, "systemctl", "stop", OpenVPNServiceName)
	return err
}

func (m *VPNManager) Status(ctx context.Context) (VPNStatus, error) {
	serviceOutput, err := m.runner.Run(ctx, "systemctl", "is-active", OpenVPNServiceName)
	if err != nil && !isInactiveService(err) {
		return VPNStatus{}, err
	}

	status := VPNStatus{
		ServiceActive: strings.TrimSpace(serviceOutput) == "active",
	}

	ipOutput, err := m.runner.Run(ctx, "ip", "-o", "-4", "addr", "show", "dev", OpenVPNInterfaceName)
	if err == nil {
		status.InterfacePresent = true
		status.IPAddress = parseInterfaceIPv4(ipOutput)
	} else if !isMissingInterface(err) {
		return VPNStatus{}, err
	}

	payload, err := m.readFile(OpenVPNStatusFilePath)
	if err == nil && strings.TrimSpace(string(payload)) != "" {
		status.StatusFilePresent = true
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return VPNStatus{}, fmt.Errorf("read vpn status file: %w", err)
	}

	if status.ServiceActive && status.InterfacePresent && status.IPAddress != "" {
		status.State = StateConnected
	} else {
		status.State = StateDisconnected
	}

	return status, nil
}

type Cleaner struct {
	wifi *WiFiManager
	vpn  *VPNManager
}

func NewCleaner(wifi *WiFiManager, vpn *VPNManager) *Cleaner {
	return &Cleaner{
		wifi: wifi,
		vpn:  vpn,
	}
}

func (c *Cleaner) Cleanup(ctx context.Context) error {
	var errs []error

	if c.vpn != nil {
		if err := c.vpn.Stop(ctx); err != nil && !isInactiveService(err) {
			errs = append(errs, err)
		}
	}

	if c.wifi != nil {
		if err := c.wifi.Disconnect(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func parseInterfaceIPv4(output string) string {
	fields := strings.Fields(output)
	for i, field := range fields {
		if field == "inet" && i+1 < len(fields) {
			return strings.Split(fields[i+1], "/")[0]
		}
	}
	return ""
}

func parseNMCLIConnection(line string) (string, string, bool) {
	split := -1
	escaped := false
	for i := len(line) - 1; i >= 0; i-- {
		switch {
		case escaped:
			escaped = false
		case line[i] == '\\':
			escaped = true
		case line[i] == ':':
			split = i
			i = -1
		}
	}

	if split <= 0 || split >= len(line)-1 {
		return "", "", false
	}

	name := strings.TrimSpace(strings.ReplaceAll(line[:split], `\:`, `:`))
	connectionType := strings.TrimSpace(strings.ReplaceAll(line[split+1:], `\:`, `:`))
	if name == "" || connectionType == "" {
		return "", "", false
	}

	return name, connectionType, true
}

func isWiFiConnectionType(connectionType string) bool {
	switch connectionType {
	case "wifi", "802-11-wireless", "wireless":
		return true
	default:
		return false
	}
}

func isMissingConnection(err error) bool {
	return strings.Contains(err.Error(), "unknown connection") || strings.Contains(err.Error(), "not found")
}

func isInactiveService(err error) bool {
	return strings.Contains(err.Error(), "inactive") || strings.Contains(err.Error(), "failed") || strings.Contains(err.Error(), "unknown")
}

func isMissingInterface(err error) bool {
	return strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "Cannot find device")
}

func IsCommandUnavailable(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "executable file not found")
}

func IsEOF(err error) bool {
	return errors.Is(err, io.EOF)
}

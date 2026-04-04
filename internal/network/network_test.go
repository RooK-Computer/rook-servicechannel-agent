package network

import (
	"context"
	"errors"
	"os"
	"testing"
)

type fakeRunner struct {
	outputs map[string]string
	errors  map[string]error
	calls   []string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	call := name + " " + joinArgs(args)
	r.calls = append(r.calls, call)
	if err, ok := r.errors[call]; ok {
		return "", err
	}
	return r.outputs[call], nil
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}

func TestWiFiScanParsesNetworks(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string]string{
			"nmcli --terse --fields SSID dev wifi list --rescan yes": "Cafe\nHome\nCafe\n\n",
		},
	}

	networks, err := NewWiFiManager(runner).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() returned error: %v", err)
	}

	if len(networks) != 2 {
		t.Fatalf("len(networks) = %d, want 2", len(networks))
	}
}

func TestWiFiConnectReconnectsSupportProfile(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string]string{
			"nmcli dev wifi connect Test password secret name rook-support-wifi": "",
		},
		errors: map[string]error{
			"nmcli connection delete rook-support-wifi": errors.New("unknown connection"),
		},
	}

	if err := NewWiFiManager(runner).Connect(context.Background(), "Test", "secret"); err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("len(calls) = %d, want 2", len(runner.calls))
	}
}

func TestWiFiStatusDetectsActiveSupportProfile(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string]string{
			"nmcli --terse --fields NAME,TYPE connection show --active": "rook-support-wifi:802-11-wireless\n",
		},
	}

	status, err := NewWiFiManager(runner).Status(context.Background())
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	if status.State != StateConnected {
		t.Fatalf("State = %q, want connected", status.State)
	}

	if !status.AnyActive {
		t.Fatal("AnyActive = false, want true")
	}

	if !status.SupportActive {
		t.Fatal("SupportActive = false, want true")
	}

	if status.ActiveConnectionName != SupportConnectionName {
		t.Fatalf("ActiveConnectionName = %q, want %q", status.ActiveConnectionName, SupportConnectionName)
	}
}

func TestWiFiStatusDetectsForeignActiveWiFi(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string]string{
			"nmcli --terse --fields NAME,TYPE connection show --active": "HomeNetwork:802-11-wireless\nbridge0:bridge\n",
		},
	}

	status, err := NewWiFiManager(runner).Status(context.Background())
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	if status.State != StateDisconnected {
		t.Fatalf("State = %q, want disconnected", status.State)
	}

	if !status.AnyActive {
		t.Fatal("AnyActive = false, want true")
	}

	if status.SupportActive {
		t.Fatal("SupportActive = true, want false")
	}

	if status.ActiveConnectionName != "HomeNetwork" {
		t.Fatalf("ActiveConnectionName = %q, want HomeNetwork", status.ActiveConnectionName)
	}
}

func TestWiFiStatusUnescapesConnectionNames(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string]string{
			"nmcli --terse --fields NAME,TYPE connection show --active": "Office\\:Guest:802-11-wireless\n",
		},
	}

	status, err := NewWiFiManager(runner).Status(context.Background())
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	if !status.AnyActive {
		t.Fatal("AnyActive = false, want true")
	}

	if status.ActiveConnectionName != "Office:Guest" {
		t.Fatalf("ActiveConnectionName = %q, want Office:Guest", status.ActiveConnectionName)
	}
}

func TestVPNStatusUsesServiceInterfaceAndStatusFile(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string]string{
			"systemctl is-active rook-openvpn-client.service": "active\n",
			"ip -o -4 addr show dev rookvpn":                  "7: rookvpn    inet 10.8.0.2/24 scope global rookvpn\n",
		},
	}

	manager := NewVPNManager(runner)
	manager.readFile = func(string) ([]byte, error) {
		return []byte("CLIENT_LIST"), nil
	}

	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	if status.State != StateConnected {
		t.Fatalf("State = %q, want connected", status.State)
	}

	if status.IPAddress != "10.8.0.2" {
		t.Fatalf("IPAddress = %q, want 10.8.0.2", status.IPAddress)
	}
}

func TestCleanerStopsVPNAndDeletesSupportProfile(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string]string{
			"systemctl stop rook-openvpn-client.service": "",
			"nmcli connection delete rook-support-wifi":  "",
		},
	}

	cleaner := NewCleaner(NewWiFiManager(runner), NewVPNManager(runner))
	if err := cleaner.Cleanup(context.Background()); err != nil {
		t.Fatalf("Cleanup() returned error: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("len(calls) = %d, want 2", len(runner.calls))
	}
}

func TestVPNStatusIgnoresMissingStatusFile(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string]string{
			"systemctl is-active rook-openvpn-client.service": "inactive\n",
		},
		errors: map[string]error{
			"ip -o -4 addr show dev rookvpn": errors.New("Cannot find device"),
		},
	}

	manager := NewVPNManager(runner)
	manager.readFile = func(string) ([]byte, error) {
		return nil, os.ErrNotExist
	}

	status, err := manager.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	if status.State != StateDisconnected {
		t.Fatalf("State = %q, want disconnected", status.State)
	}
}

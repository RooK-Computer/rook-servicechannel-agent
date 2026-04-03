package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"rook-servicechannel-agent/internal/backend"
	agentruntime "rook-servicechannel-agent/internal/runtime"
)

type Action string

const (
	GetStatusAction      Action = "GetStatus"
	ScanWiFiAction       Action = "ScanWifi"
	ConnectWiFiAction    Action = "ConnectWifi"
	DisconnectWiFiAction Action = "DisconnectWifi"
	StartSupportAction   Action = "StartSupport"
	StopSupportAction    Action = "StopSupport"
	GetPinAction         Action = "GetPin"
)

const (
	messageTypeResponse = "response"
	messageTypeEvent    = "event"
)

const (
	SupportStateChangedEvent        = "SupportStateChanged"
	WiFiScanCompletedEvent          = "WifiScanCompleted"
	WiFiConnectionStateChangedEvent = "WifiConnectionStateChanged"
	VPNStateChangedEvent            = "VpnStateChanged"
	PinAssignedEvent                = "PinAssigned"
	ErrorRaisedEvent                = "ErrorRaised"
)

type Request struct {
	ID      string          `json:"id"`
	Action  Action          `json:"action"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func (r Request) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("request id must not be empty")
	}

	switch r.Action {
	case GetStatusAction, ScanWiFiAction, ConnectWiFiAction, DisconnectWiFiAction, StartSupportAction, StopSupportAction, GetPinAction:
		return nil
	default:
		return fmt.Errorf("unsupported action %q", r.Action)
	}
}

type Response struct {
	Type    string        `json:"type"`
	ID      string        `json:"id"`
	Action  Action        `json:"action"`
	Success bool          `json:"success"`
	Payload interface{}   `json:"payload,omitempty"`
	Error   *ErrorPayload `json:"error,omitempty"`
}

type Event struct {
	Type    string      `json:"type"`
	Event   string      `json:"event"`
	Payload interface{} `json:"payload,omitempty"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StatusPayload struct {
	SupportActive        bool                 `json:"supportActive"`
	SupportState         string               `json:"supportState"`
	WiFiState            string               `json:"wifiState"`
	VPNState             string               `json:"vpnState"`
	AnyWiFiActive        bool                 `json:"anyWifiActive"`
	SupportWiFiActive    bool                 `json:"supportWifiActive"`
	ActiveWiFiConnection string               `json:"activeWifiConnection,omitempty"`
	Networks             []WiFiNetworkPayload `json:"networks,omitempty"`
	Session              *SessionPayload      `json:"session,omitempty"`
}

type SessionPayload struct {
	Status    string `json:"status"`
	PIN       string `json:"pin"`
	IPAddress string `json:"ipAddress"`
}

type PinPayload struct {
	PIN string `json:"pin"`
}

type ConnectWiFiPayload struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

type WiFiNetworkPayload struct {
	SSID string `json:"ssid"`
}

type WiFiScanPayload struct {
	Networks []WiFiNetworkPayload `json:"networks"`
}

type ConnectionStatePayload struct {
	State string `json:"state"`
}

func NewStatusPayload(snapshot agentruntime.Snapshot) StatusPayload {
	networks := make([]WiFiNetworkPayload, 0, len(snapshot.WiFiNetworks))
	for _, network := range snapshot.WiFiNetworks {
		networks = append(networks, WiFiNetworkPayload{SSID: network.SSID})
	}

	if !snapshot.HasSession {
		return StatusPayload{
			SupportActive:        false,
			SupportState:         string(snapshot.SupportState),
			WiFiState:            string(snapshot.WiFiState),
			VPNState:             string(snapshot.VPNState),
			AnyWiFiActive:        snapshot.AnyWiFiActive,
			SupportWiFiActive:    snapshot.SupportWiFiActive,
			ActiveWiFiConnection: snapshot.ActiveWiFiConnection,
			Networks:             networks,
		}
	}

	return StatusPayload{
		SupportActive:        true,
		SupportState:         string(snapshot.SupportState),
		WiFiState:            string(snapshot.WiFiState),
		VPNState:             string(snapshot.VPNState),
		AnyWiFiActive:        snapshot.AnyWiFiActive,
		SupportWiFiActive:    snapshot.SupportWiFiActive,
		ActiveWiFiConnection: snapshot.ActiveWiFiConnection,
		Networks:             networks,
		Session:              NewSessionPayload(snapshot.Session),
	}
}

func NewSessionPayload(session backend.SupportSession) *SessionPayload {
	return &SessionPayload{
		Status:    string(session.Status),
		PIN:       session.PIN,
		IPAddress: session.IPAddress,
	}
}

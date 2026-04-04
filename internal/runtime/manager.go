package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"rook-servicechannel-agent/internal/backend"
	"rook-servicechannel-agent/internal/host"
	"rook-servicechannel-agent/internal/sessionstate"
)

const serviceShutdownTimeout = 5 * time.Second

type EventKind string

const (
	EventHeartbeatStarted  EventKind = "heartbeat_started"
	EventHeartbeatStopped  EventKind = "heartbeat_stopped"
	EventHeartbeatError    EventKind = "heartbeat_error"
	EventHeartbeatFatal    EventKind = "heartbeat_fatal"
	EventSessionResumed    EventKind = "session_resumed"
	EventSessionEnded      EventKind = "session_ended"
	EventWiFiScanCompleted EventKind = "wifi_scan_completed"
	EventWiFiStateChanged  EventKind = "wifi_state_changed"
	EventVPNStateChanged   EventKind = "vpn_state_changed"
)

type BinaryState string

const (
	BinaryStateConnected    BinaryState = "connected"
	BinaryStateDisconnected BinaryState = "disconnected"
)

type SupportState string

const (
	SupportStateIdle        SupportState = "idle"
	SupportStateOnline      SupportState = "online"
	SupportStateOnlineVPNUp SupportState = "online+vpnup"
	SupportStateServiceMode SupportState = "servicemode"
)

type WiFiNetwork struct {
	SSID string
}

type Event struct {
	Kind     EventKind
	PIN      string
	Interval time.Duration
	Err      error
	State    string
	Networks []WiFiNetwork
}

type EventHandler func(Event)

type Snapshot struct {
	HasSession           bool
	Session              backend.SupportSession
	WiFiState            BinaryState
	VPNState             BinaryState
	SupportState         SupportState
	WiFiNetworks         []WiFiNetwork
	AnyWiFiActive        bool
	SupportWiFiActive    bool
	ActiveWiFiConnection string
}

type Manager struct {
	backendURL string
	stateStore sessionstate.Store
	bootIDFunc func() (string, error)
	logger     *slog.Logger

	heartbeatInterval time.Duration

	heartbeatMu          sync.Mutex
	heartbeatCancel      context.CancelFunc
	networkMu            sync.Mutex
	wifiState            BinaryState
	vpnState             BinaryState
	wifiNetworks         []WiFiNetwork
	anyWiFiActive        bool
	supportWiFiActive    bool
	activeWiFiConnection string
	subscribersMu        sync.Mutex
	subscribers          map[int]EventHandler
	nextSubscriberID     int
}

func New(backendURL, statePath string) *Manager {
	return &Manager{
		backendURL:        backendURL,
		stateStore:        sessionstate.New(statePath),
		bootIDFunc:        host.CurrentBootID,
		heartbeatInterval: backend.HeartbeatFrequency,
		heartbeatCancel:   nil,
		wifiState:         BinaryStateDisconnected,
		vpnState:          BinaryStateDisconnected,
		subscribers:       make(map[int]EventHandler),
	}
}

func (m *Manager) Subscribe(handler EventHandler) func() {
	if handler == nil {
		return func() {}
	}

	m.subscribersMu.Lock()
	id := m.nextSubscriberID
	m.nextSubscriberID++
	m.subscribers[id] = handler
	m.subscribersMu.Unlock()

	return func() {
		m.subscribersMu.Lock()
		delete(m.subscribers, id)
		m.subscribersMu.Unlock()
	}
}

func (m *Manager) SetHeartbeatInterval(interval time.Duration) {
	m.heartbeatMu.Lock()
	defer m.heartbeatMu.Unlock()
	m.heartbeatInterval = interval
}

func (m *Manager) SetLogger(logger *slog.Logger) {
	m.logger = logger
}

func (m *Manager) BeginSession(ctx context.Context) (backend.SupportSession, error) {
	client, err := m.client()
	if err != nil {
		return backend.SupportSession{}, err
	}

	response, err := client.BeginSession(ctx, backend.StartSupportSessionRequest{})
	if err != nil {
		return backend.SupportSession{}, err
	}

	if err := m.saveSession(response.Session); err != nil {
		return backend.SupportSession{}, err
	}

	return response.Session, nil
}

func (m *Manager) GetSessionStatus(ctx context.Context, pinOverride string) (backend.SupportSession, error) {
	client, err := m.client()
	if err != nil {
		return backend.SupportSession{}, err
	}

	pin, err := m.resolvePIN(pinOverride)
	if err != nil {
		return backend.SupportSession{}, err
	}

	response, err := client.GetSessionStatus(ctx, backend.SessionStatusRequest{PIN: pin})
	if err != nil {
		return backend.SupportSession{}, err
	}

	if err := m.saveSession(response.Session); err != nil {
		return backend.SupportSession{}, err
	}

	return response.Session, nil
}

func (m *Manager) CurrentPIN(pinOverride string) (string, error) {
	return m.resolvePIN(pinOverride)
}

func (m *Manager) Snapshot() (Snapshot, error) {
	state, err := m.stateStore.Load()
	if err != nil {
		if errors.Is(err, sessionstate.ErrStateNotFound) {
			wifiState := m.currentWiFiState()
			vpnState := m.currentVPNState()
			return Snapshot{
				WiFiState:            wifiState,
				VPNState:             vpnState,
				SupportState:         m.deriveSupportState(false, wifiState, vpnState),
				WiFiNetworks:         m.currentWiFiNetworks(),
				AnyWiFiActive:        m.currentAnyWiFiActive(),
				SupportWiFiActive:    m.currentSupportWiFiActive(),
				ActiveWiFiConnection: m.currentActiveWiFiConnection(),
			}, nil
		}
		return Snapshot{}, err
	}

	wifiState := m.currentWiFiState()
	vpnState := m.currentVPNState()
	return Snapshot{
		HasSession:           true,
		Session:              state.Session,
		WiFiState:            wifiState,
		VPNState:             vpnState,
		SupportState:         m.deriveSupportState(true, wifiState, vpnState),
		WiFiNetworks:         m.currentWiFiNetworks(),
		AnyWiFiActive:        m.currentAnyWiFiActive(),
		SupportWiFiActive:    m.currentSupportWiFiActive(),
		ActiveWiFiConnection: m.currentActiveWiFiConnection(),
	}, nil
}

func (m *Manager) UpdateWiFiNetworks(networks []WiFiNetwork) {
	m.syncWiFiNetworks(networks)
	m.emit(Event{Kind: EventWiFiScanCompleted, Networks: append([]WiFiNetwork(nil), networks...)})
}

func (m *Manager) SyncWiFiNetworks(networks []WiFiNetwork) {
	m.syncWiFiNetworks(networks)
}

func (m *Manager) syncWiFiNetworks(networks []WiFiNetwork) {
	m.networkMu.Lock()
	copied := append([]WiFiNetwork(nil), networks...)
	m.wifiNetworks = copied
	m.networkMu.Unlock()
}

func (m *Manager) SetWiFiState(state BinaryState) {
	m.syncWiFiState(state)
	m.emit(Event{Kind: EventWiFiStateChanged, State: string(state)})
}

func (m *Manager) SyncWiFiState(state BinaryState) {
	m.syncWiFiState(state)
}

func (m *Manager) syncWiFiState(state BinaryState) {
	m.networkMu.Lock()
	m.wifiState = state
	m.networkMu.Unlock()
}

func (m *Manager) SetWiFiStatus(anyActive, supportActive bool, activeConnection string) {
	m.syncWiFiStatus(anyActive, supportActive, activeConnection)
}

func (m *Manager) SyncWiFiStatus(anyActive, supportActive bool, activeConnection string) {
	m.syncWiFiStatus(anyActive, supportActive, activeConnection)
}

func (m *Manager) syncWiFiStatus(anyActive, supportActive bool, activeConnection string) {
	m.networkMu.Lock()
	m.anyWiFiActive = anyActive
	m.supportWiFiActive = supportActive
	m.activeWiFiConnection = activeConnection
	m.networkMu.Unlock()
}

func (m *Manager) SetVPNState(state BinaryState) {
	m.syncVPNState(state)
	m.emit(Event{Kind: EventVPNStateChanged, State: string(state)})
}

func (m *Manager) SyncVPNState(state BinaryState) {
	m.syncVPNState(state)
}

func (m *Manager) syncVPNState(state BinaryState) {
	m.networkMu.Lock()
	m.vpnState = state
	m.networkMu.Unlock()
}

func (m *Manager) RecoverAfterBoot() (bool, error) {
	state, err := m.stateStore.Load()
	if err != nil {
		if errors.Is(err, sessionstate.ErrStateNotFound) {
			return false, nil
		}
		return false, err
	}

	currentBootID, err := m.currentBootID()
	if err != nil {
		return false, err
	}
	if state.BootID == "" || state.BootID == currentBootID {
		return false, nil
	}

	if err := m.stateStore.Clear(); err != nil {
		return false, err
	}

	m.emit(Event{Kind: EventSessionEnded, PIN: state.Session.PIN})
	return true, nil
}

func (m *Manager) SendHeartbeat(ctx context.Context, pinOverride string) error {
	pin, err := m.resolvePIN(pinOverride)
	if err != nil {
		return err
	}

	return m.sendHeartbeatForPIN(ctx, pin)
}

func (m *Manager) StopSession(ctx context.Context, pinOverride string) error {
	pin, err := m.resolvePIN(pinOverride)
	if err != nil {
		return err
	}

	m.StopHeartbeatLoop()
	if err := m.endSessionForPIN(ctx, pin); err != nil {
		return err
	}

	m.emit(Event{Kind: EventSessionEnded, PIN: pin})
	return nil
}

func (m *Manager) StartHeartbeatLoop(parent context.Context, pin string) {
	m.StopHeartbeatLoop()

	ctx, cancel := context.WithCancel(parent)

	m.heartbeatMu.Lock()
	m.heartbeatCancel = cancel
	interval := m.heartbeatInterval
	m.heartbeatMu.Unlock()

	m.emit(Event{
		Kind:     EventHeartbeatStarted,
		PIN:      pin,
		Interval: interval,
	})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.sendHeartbeatForPIN(ctx, pin); err != nil {
					var requestErr *backend.RequestError
					if errors.As(err, &requestErr) {
						m.emit(Event{Kind: EventHeartbeatFatal, PIN: pin, Err: err})
						m.StopHeartbeatLoop()
						return
					}

					m.emit(Event{Kind: EventHeartbeatError, PIN: pin, Err: err})
				}
			}
		}
	}()
}

func (m *Manager) StopHeartbeatLoop() {
	m.heartbeatMu.Lock()
	cancel := m.heartbeatCancel
	hadHeartbeat := cancel != nil
	m.heartbeatCancel = nil
	m.heartbeatMu.Unlock()

	if cancel != nil {
		cancel()
	}

	if hadHeartbeat {
		m.emit(Event{Kind: EventHeartbeatStopped})
	}
}

func (m *Manager) RunService(ctx context.Context) error {
	state, err := m.stateStore.Load()
	switch {
	case err == nil:
		m.emit(Event{Kind: EventSessionResumed, PIN: state.Session.PIN})
		m.StartHeartbeatLoop(ctx, state.Session.PIN)
	case errors.Is(err, sessionstate.ErrStateNotFound):
	default:
		return err
	}

	<-ctx.Done()

	state, err = m.stateStore.Load()
	if err != nil {
		m.StopHeartbeatLoop()
		if errors.Is(err, sessionstate.ErrStateNotFound) {
			return nil
		}
		return err
	}

	m.StopHeartbeatLoop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), serviceShutdownTimeout)
	defer cancel()

	if err := m.endSessionForPIN(shutdownCtx, state.Session.PIN); err != nil {
		return fmt.Errorf("shutdown active session: %w", err)
	}

	m.emit(Event{Kind: EventSessionEnded, PIN: state.Session.PIN})
	return nil
}

func (m *Manager) client() (backend.Client, error) {
	return backend.NewClientWithLogger(m.backendURL, nil, m.logger)
}

func (m *Manager) sendHeartbeatForPIN(ctx context.Context, pin string) error {
	client, err := m.client()
	if err != nil {
		return err
	}

	_, err = client.SendSessionHeartbeat(ctx, backend.SessionHeartbeatRequest{PIN: pin})
	return err
}

func (m *Manager) endSessionForPIN(ctx context.Context, pin string) error {
	client, err := m.client()
	if err != nil {
		return err
	}

	if _, err := client.EndSession(ctx, backend.EndSupportSessionRequest{PIN: pin}); err != nil {
		return err
	}

	if err := m.stateStore.Clear(); err != nil {
		return err
	}

	return nil
}

func (m *Manager) currentBootID() (string, error) {
	if m.bootIDFunc == nil {
		return "", nil
	}
	return m.bootIDFunc()
}

func (m *Manager) saveSession(session backend.SupportSession) error {
	bootID, err := m.currentBootID()
	if err != nil {
		return err
	}

	return m.stateStore.Save(sessionstate.State{
		Session: session,
		BootID:  bootID,
	})
}

func (m *Manager) currentWiFiState() BinaryState {
	m.networkMu.Lock()
	defer m.networkMu.Unlock()
	return m.wifiState
}

func (m *Manager) currentVPNState() BinaryState {
	m.networkMu.Lock()
	defer m.networkMu.Unlock()
	return m.vpnState
}

func (m *Manager) currentWiFiNetworks() []WiFiNetwork {
	m.networkMu.Lock()
	defer m.networkMu.Unlock()
	return append([]WiFiNetwork(nil), m.wifiNetworks...)
}

func (m *Manager) currentAnyWiFiActive() bool {
	m.networkMu.Lock()
	defer m.networkMu.Unlock()
	return m.anyWiFiActive
}

func (m *Manager) currentSupportWiFiActive() bool {
	m.networkMu.Lock()
	defer m.networkMu.Unlock()
	return m.supportWiFiActive
}

func (m *Manager) currentActiveWiFiConnection() string {
	m.networkMu.Lock()
	defer m.networkMu.Unlock()
	return m.activeWiFiConnection
}

func (m *Manager) deriveSupportState(hasSession bool, wifiState, vpnState BinaryState) SupportState {
	switch {
	case hasSession:
		return SupportStateServiceMode
	case vpnState == BinaryStateConnected:
		return SupportStateOnlineVPNUp
	case wifiState == BinaryStateConnected:
		return SupportStateOnline
	default:
		return SupportStateIdle
	}
}

func (m *Manager) resolvePIN(pinOverride string) (string, error) {
	if pinOverride != "" {
		return pinOverride, nil
	}

	state, err := m.stateStore.Load()
	if err != nil {
		if errors.Is(err, sessionstate.ErrStateNotFound) {
			return "", errors.New("no active session state found; use start first or provide --pin")
		}
		return "", err
	}

	return state.Session.PIN, nil
}

func (m *Manager) emit(event Event) {
	m.subscribersMu.Lock()
	handlers := make([]EventHandler, 0, len(m.subscribers))
	for _, handler := range m.subscribers {
		handlers = append(handlers, handler)
	}
	m.subscribersMu.Unlock()

	for _, handler := range handlers {
		handler(event)
	}
}

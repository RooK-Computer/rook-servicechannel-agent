package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	"rook-servicechannel-agent/internal/network"
	agentruntime "rook-servicechannel-agent/internal/runtime"
)

type Server struct {
	socketPath string
	logger     *slog.Logger
	manager    *agentruntime.Manager
	wifi       *network.WiFiManager
	vpn        *network.VPNManager

	clientsMu sync.Mutex
	clients   map[int]*client
	nextID    int
}

type client struct {
	id       int
	conn     net.Conn
	outbound chan interface{}
}

func NewServer(socketPath string, logger *slog.Logger, manager *agentruntime.Manager, wifi *network.WiFiManager, vpn *network.VPNManager) *Server {
	return &Server{
		socketPath: socketPath,
		logger:     logger,
		manager:    manager,
		wifi:       wifi,
		vpn:        vpn,
		clients:    make(map[int]*client),
	}
}

func (s *Server) Run(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o700); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	if err := removeStaleSocket(s.socketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on socket: %w", err)
	}
	defer listener.Close()
	defer os.Remove(s.socketPath)

	if err := os.Chmod(s.socketPath, 0o600); err != nil {
		return fmt.Errorf("chmod socket: %w", err)
	}

	unsubscribe := s.manager.Subscribe(s.handleRuntimeEvent)
	defer unsubscribe()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
		s.closeAllClients()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept socket client: %w", err)
		}

		go s.handleClient(ctx, conn)
	}
}

func (s *Server) handleClient(ctx context.Context, conn net.Conn) {
	client := s.addClient(conn)
	defer s.removeClient(client.id)

	go s.writeLoop(client)

	decoder := json.NewDecoder(conn)
	for {
		var request Request
		if err := decoder.Decode(&request); err != nil {
			if !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.Canceled) && !isEOF(err) {
				s.logger.Warn("ipc client decode failed", "error", err)
			}
			return
		}

		response, events := s.handleRequest(ctx, request)
		client.outbound <- response
		for _, event := range events {
			s.broadcast(event)
		}
	}
}

func (s *Server) writeLoop(client *client) {
	encoder := json.NewEncoder(client.conn)
	for message := range client.outbound {
		if err := encoder.Encode(message); err != nil {
			return
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, request Request) (Response, []Event) {
	if err := request.Validate(); err != nil {
		return errorResponse(request, "invalid_request", err.Error()), nil
	}

	switch request.Action {
	case GetStatusAction:
		wifiStatus, err := s.wifi.Status(ctx)
		if err != nil {
			return errorResponse(request, "status_failed", err.Error()), nil
		}
		s.manager.SyncWiFiState(agentruntime.BinaryState(wifiStatus.State))
		s.manager.SyncWiFiStatus(wifiStatus.AnyActive, wifiStatus.SupportActive, wifiStatus.ActiveConnectionName)

		vpnStatus, err := s.vpn.Status(ctx)
		if err != nil {
			return errorResponse(request, "status_failed", err.Error()), nil
		}
		s.manager.SyncVPNState(agentruntime.BinaryState(vpnStatus.State))

		snapshot, err := s.manager.Snapshot()
		if err != nil {
			return errorResponse(request, "status_failed", err.Error()), nil
		}
		return successResponse(request, NewStatusPayload(snapshot)), nil
	case PingAction:
		if err := s.manager.SendHeartbeat(ctx, ""); err != nil {
			return errorResponse(request, "ping_failed", err.Error()), []Event{errorEvent(err)}
		}
		return successResponse(request, struct{}{}), nil
	case ScanWiFiAction:
		networksFound, err := s.wifi.Scan(ctx)
		if err != nil {
			return errorResponse(request, "scan_wifi_failed", err.Error()), []Event{errorEvent(err)}
		}
		runtimeNetworks := make([]agentruntime.WiFiNetwork, 0, len(networksFound))
		payloadNetworks := make([]WiFiNetworkPayload, 0, len(networksFound))
		for _, networkFound := range networksFound {
			runtimeNetworks = append(runtimeNetworks, agentruntime.WiFiNetwork{SSID: networkFound.SSID})
			payloadNetworks = append(payloadNetworks, WiFiNetworkPayload{SSID: networkFound.SSID})
		}
		s.manager.SyncWiFiNetworks(runtimeNetworks)
		return successResponse(request, WiFiScanPayload{Networks: payloadNetworks}), []Event{
			{Type: messageTypeEvent, Event: WiFiScanCompletedEvent, Payload: WiFiScanPayload{Networks: payloadNetworks}},
		}
	case ConnectWiFiAction:
		var payload ConnectWiFiPayload
		if err := json.Unmarshal(request.Payload, &payload); err != nil {
			return errorResponse(request, "invalid_payload", err.Error()), nil
		}
		if err := s.wifi.Connect(ctx, payload.SSID, payload.Password); err != nil {
			return errorResponse(request, "connect_wifi_failed", err.Error()), []Event{errorEvent(err)}
		}
		s.manager.SyncWiFiState(agentruntime.BinaryStateConnected)
		s.manager.SyncWiFiStatus(true, true, network.SupportConnectionName)
		return successResponse(request, ConnectionStatePayload{State: string(agentruntime.BinaryStateConnected)}), []Event{
			{Type: messageTypeEvent, Event: WiFiConnectionStateChangedEvent, Payload: ConnectionStatePayload{State: string(agentruntime.BinaryStateConnected)}},
		}
	case DisconnectWiFiAction:
		if err := s.wifi.Disconnect(ctx); err != nil {
			return errorResponse(request, "disconnect_wifi_failed", err.Error()), []Event{errorEvent(err)}
		}
		s.manager.SyncWiFiState(agentruntime.BinaryStateDisconnected)
		s.manager.SyncWiFiStatus(false, false, "")
		return successResponse(request, ConnectionStatePayload{State: string(agentruntime.BinaryStateDisconnected)}), []Event{
			{Type: messageTypeEvent, Event: WiFiConnectionStateChangedEvent, Payload: ConnectionStatePayload{State: string(agentruntime.BinaryStateDisconnected)}},
		}
	case VPNStatusAction:
		status, err := s.vpn.Status(ctx)
		if err != nil {
			return errorResponse(request, "vpn_status_failed", err.Error()), []Event{errorEvent(err)}
		}
		s.manager.SyncVPNState(agentruntime.BinaryState(status.State))
		return successResponse(request, NewVPNStatusPayload(status)), nil
	case VPNStartAction:
		if err := s.vpn.Start(ctx); err != nil {
			return errorResponse(request, "vpn_start_failed", err.Error()), []Event{errorEvent(err)}
		}
		status, err := s.vpn.Status(ctx)
		if err != nil {
			return errorResponse(request, "vpn_status_failed", err.Error()), []Event{errorEvent(err)}
		}
		s.manager.SyncVPNState(agentruntime.BinaryState(status.State))
		return successResponse(request, NewVPNStatusPayload(status)), []Event{
			{Type: messageTypeEvent, Event: VPNStateChangedEvent, Payload: ConnectionStatePayload{State: string(status.State)}},
		}
	case VPNStopAction:
		if err := s.vpn.Stop(ctx); err != nil {
			return errorResponse(request, "vpn_stop_failed", err.Error()), []Event{errorEvent(err)}
		}
		s.manager.SyncVPNState(agentruntime.BinaryStateDisconnected)
		return successResponse(request, ConnectionStatePayload{State: string(agentruntime.BinaryStateDisconnected)}), []Event{
			{Type: messageTypeEvent, Event: VPNStateChangedEvent, Payload: ConnectionStatePayload{State: string(agentruntime.BinaryStateDisconnected)}},
		}
	case CleanupAction:
		cleaner := network.NewCleaner(s.wifi, s.vpn)
		if err := cleaner.Cleanup(ctx); err != nil {
			return errorResponse(request, "cleanup_failed", err.Error()), []Event{errorEvent(err)}
		}
		s.manager.SyncWiFiState(agentruntime.BinaryStateDisconnected)
		s.manager.SyncWiFiStatus(false, false, "")
		s.manager.SyncVPNState(agentruntime.BinaryStateDisconnected)
		return successResponse(request, struct{}{}), []Event{
			{Type: messageTypeEvent, Event: WiFiConnectionStateChangedEvent, Payload: ConnectionStatePayload{State: string(agentruntime.BinaryStateDisconnected)}},
			{Type: messageTypeEvent, Event: VPNStateChangedEvent, Payload: ConnectionStatePayload{State: string(agentruntime.BinaryStateDisconnected)}},
		}
	case StartSupportAction:
		session, err := s.manager.BeginSession(ctx)
		if err != nil {
			return errorResponse(request, "start_support_failed", err.Error()), []Event{errorEvent(err)}
		}
		s.manager.StartHeartbeatLoop(ctx, session.PIN)
		snapshot, err := s.manager.Snapshot()
		if err != nil {
			return errorResponse(request, "status_failed", err.Error()), []Event{errorEvent(err)}
		}
		return successResponse(request, NewStatusPayload(snapshot)), []Event{
			supportStateEvent(snapshot),
			{
				Type:  messageTypeEvent,
				Event: PinAssignedEvent,
				Payload: PinPayload{
					PIN: session.PIN,
				},
			},
		}
	case StopSupportAction:
		if err := s.manager.StopSession(ctx, ""); err != nil {
			return errorResponse(request, "stop_support_failed", err.Error()), []Event{errorEvent(err)}
		}
		snapshot, err := s.manager.Snapshot()
		if err != nil {
			return errorResponse(request, "status_failed", err.Error()), []Event{errorEvent(err)}
		}
		return successResponse(request, NewStatusPayload(snapshot)), nil
	case GetPinAction:
		pin, err := s.manager.CurrentPIN("")
		if err != nil {
			return errorResponse(request, "get_pin_failed", err.Error()), nil
		}
		return successResponse(request, PinPayload{PIN: pin}), nil
	default:
		return errorResponse(request, "unsupported_action", fmt.Sprintf("unsupported action %q", request.Action)), nil
	}
}

func (s *Server) handleRuntimeEvent(event agentruntime.Event) {
	switch event.Kind {
	case agentruntime.EventSessionResumed:
		snapshot, err := s.manager.Snapshot()
		if err != nil {
			s.broadcast(errorEvent(err))
			return
		}
		s.broadcast(supportStateEvent(snapshot))
	case agentruntime.EventSessionEnded:
		s.broadcast(supportStateEvent(agentruntime.Snapshot{}))
	case agentruntime.EventWiFiScanCompleted:
		networks := make([]WiFiNetworkPayload, 0, len(event.Networks))
		for _, network := range event.Networks {
			networks = append(networks, WiFiNetworkPayload{SSID: network.SSID})
		}
		s.broadcast(Event{Type: messageTypeEvent, Event: WiFiScanCompletedEvent, Payload: WiFiScanPayload{Networks: networks}})
	case agentruntime.EventWiFiStateChanged:
		s.broadcast(Event{Type: messageTypeEvent, Event: WiFiConnectionStateChangedEvent, Payload: ConnectionStatePayload{State: event.State}})
	case agentruntime.EventVPNStateChanged:
		s.broadcast(Event{Type: messageTypeEvent, Event: VPNStateChangedEvent, Payload: ConnectionStatePayload{State: event.State}})
	case agentruntime.EventHeartbeatError:
		s.broadcast(errorEvent(event.Err))
	case agentruntime.EventHeartbeatFatal:
		s.broadcast(errorEvent(event.Err))
		s.broadcast(supportStateEvent(agentruntime.Snapshot{}))
	}
}

func (s *Server) addClient(conn net.Conn) *client {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	client := &client{
		id:       s.nextID,
		conn:     conn,
		outbound: make(chan interface{}, 16),
	}
	s.nextID++
	s.clients[client.id] = client
	return client
}

func (s *Server) removeClient(id int) {
	s.clientsMu.Lock()
	client, ok := s.clients[id]
	if ok {
		delete(s.clients, id)
	}
	s.clientsMu.Unlock()

	if ok {
		close(client.outbound)
		_ = client.conn.Close()
	}
}

func (s *Server) closeAllClients() {
	s.clientsMu.Lock()
	clients := make([]*client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	s.clients = make(map[int]*client)
	s.clientsMu.Unlock()

	for _, client := range clients {
		close(client.outbound)
		_ = client.conn.Close()
	}
}

func (s *Server) broadcast(event Event) {
	s.clientsMu.Lock()
	clients := make([]*client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	s.clientsMu.Unlock()

	for _, client := range clients {
		select {
		case client.outbound <- event:
		default:
			s.logger.Warn("dropping ipc event for slow client", "event", event.Event)
		}
	}
}

func successResponse(request Request, payload interface{}) Response {
	return Response{
		Type:    messageTypeResponse,
		ID:      request.ID,
		Action:  request.Action,
		Success: true,
		Payload: payload,
	}
}

func errorResponse(request Request, code, message string) Response {
	return Response{
		Type:    messageTypeResponse,
		ID:      request.ID,
		Action:  request.Action,
		Success: false,
		Error: &ErrorPayload{
			Code:    code,
			Message: message,
		},
	}
}

func supportStateEvent(snapshot agentruntime.Snapshot) Event {
	return Event{
		Type:    messageTypeEvent,
		Event:   SupportStateChangedEvent,
		Payload: NewStatusPayload(snapshot),
	}
}

func errorEvent(err error) Event {
	return Event{
		Type:  messageTypeEvent,
		Event: ErrorRaisedEvent,
		Payload: ErrorPayload{
			Code:    "runtime_error",
			Message: err.Error(),
		},
	}
}

func removeStaleSocket(path string) error {
	err := os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("remove stale socket: %w", err)
}

func isEOF(err error) bool {
	return errors.Is(err, os.ErrClosed) || errors.Is(err, io.EOF)
}

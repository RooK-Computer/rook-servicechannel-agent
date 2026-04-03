package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"rook-servicechannel-agent/internal/backend"
	"rook-servicechannel-agent/internal/config"
	"rook-servicechannel-agent/internal/ipc"
	"rook-servicechannel-agent/internal/network"
	agentruntime "rook-servicechannel-agent/internal/runtime"
)

type App struct {
	config config.Config
	logger *slog.Logger
	stdin  io.Reader
	stdout io.Writer

	runtimeManager *agentruntime.Manager
	wifiManager    *network.WiFiManager
	vpnManager     *network.VPNManager
	cleaner        *network.Cleaner
}

func New(cfg config.Config, logger *slog.Logger, stdin io.Reader, stdout io.Writer) App {
	if cfg.SocketPath == "" && cfg.StatePath != "" {
		cfg.SocketPath = filepath.Join(filepath.Dir(cfg.StatePath), "agent.sock")
	}

	return App{
		config:         cfg,
		logger:         logger,
		stdin:          stdin,
		stdout:         stdout,
		runtimeManager: agentruntime.New(cfg.BackendURL, cfg.StatePath),
		wifiManager:    network.NewWiFiManager(nil),
		vpnManager:     network.NewVPNManager(nil),
		cleaner:        nil,
	}
}

func (a App) Run(ctx context.Context) error {
	unsubscribe := a.runtimeManager.Subscribe(a.handleRuntimeEvent)
	defer unsubscribe()

	command := a.config.Command
	if command == "" {
		command = config.RunCommand
	}

	switch command {
	case config.RunCommand, config.ServiceCommand:
		return a.runService(ctx)
	case config.InteractiveCommand:
		return a.runInteractive(ctx)
	case config.ConfigCommand:
		fmt.Fprintln(a.stdout, a.config.Summary())
		return nil
	case config.StartCommand:
		return a.startSession(ctx)
	case config.StatusCommand:
		return a.printSessionStatus(ctx)
	case config.PinCommand:
		return a.printSessionPIN()
	case config.PingCommand:
		return a.sendHeartbeat(ctx)
	case config.StopCommand:
		return a.stopSession(ctx)
	case config.ScanWiFiCommand:
		return a.scanWiFi(ctx)
	case config.WiFiStatusCommand:
		return a.printWiFiStatus(ctx)
	case config.ConnectWiFiCommand:
		return a.connectWiFi(ctx, a.config.WiFiSSID, a.config.WiFiPassword)
	case config.DisconnectWiFiCommand:
		return a.disconnectWiFi(ctx)
	case config.VPNStatusCommand:
		return a.printVPNStatus(ctx)
	case config.VPNStartCommand:
		return a.startVPN(ctx)
	case config.VPNStopCommand:
		return a.stopVPN(ctx)
	case config.CleanupCommand:
		return a.cleanup(ctx)
	}

	return fmt.Errorf("unsupported command %q", command)
}

func (a *App) runService(ctx context.Context) error {
	serviceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if a.cleaner == nil {
		a.cleaner = network.NewCleaner(a.wifiManager, a.vpnManager)
	}

	recovered, err := a.runtimeManager.RecoverAfterBoot()
	if err != nil {
		return err
	}

	snapshot, err := a.runtimeManager.Snapshot()
	if err != nil {
		return err
	}

	if recovered || !snapshot.HasSession {
		if err := a.cleaner.Cleanup(serviceCtx); err != nil && !network.IsCommandUnavailable(err) {
			return fmt.Errorf("startup cleanup: %w", err)
		}
	}

	if err := a.syncNetworkState(serviceCtx); err != nil && !network.IsCommandUnavailable(err) {
		return err
	}

	ipcServer := ipc.NewServer(a.config.SocketPath, a.logger, a.runtimeManager, a.wifiManager, a.vpnManager)
	errCh := make(chan error, 2)

	go func() {
		errCh <- ipcServer.Run(serviceCtx)
	}()

	go func() {
		errCh <- a.runtimeManager.RunService(serviceCtx)
	}()

	a.logger.Info("rook agent service mode ready",
		"backend_url", a.config.BackendURL,
		"console_id", emptyAsUnset(a.config.ConsoleID),
		"socket_path", a.config.SocketPath,
		"mode", "service",
	)

	var firstErr error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
			cancel()
		}
	}

	if firstErr != nil {
		return firstErr
	}

	a.logger.Info("shutdown requested", "reason", serviceCtx.Err())
	return nil
}

func (a *App) runInteractive(ctx context.Context) error {
	fmt.Fprintln(a.stdout, "Entering interactive mode. Type 'help' for commands.")

	lines := make(chan string)
	scanErrors := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(a.stdin)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			scanErrors <- err
		}
		close(lines)
	}()

	for {
		fmt.Fprint(a.stdout, "rook> ")

		select {
		case <-ctx.Done():
			a.runtimeManager.StopHeartbeatLoop()
			fmt.Fprintln(a.stdout, "\ninteractive mode stopped")
			return nil
		case err := <-scanErrors:
			a.runtimeManager.StopHeartbeatLoop()
			return err
		case line, ok := <-lines:
			if !ok {
				a.runtimeManager.StopHeartbeatLoop()
				fmt.Fprintln(a.stdout, "\ninteractive mode ended")
				return nil
			}

			command := strings.TrimSpace(line)
			if command == "" {
				continue
			}

			if err := a.handleInteractiveCommand(ctx, command); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				fmt.Fprintf(a.stdout, "error: %v\n", err)
			}
		}
	}
}

func (a *App) handleInteractiveCommand(ctx context.Context, command string) error {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil
	}

	switch strings.ToLower(fields[0]) {
	case "help":
		fmt.Fprintln(a.stdout, "commands: help, config, start, status, pin, ping, stop, scanwifi, wifistatus, connectwifi <ssid> <password>, disconnectwifi, vpnstatus, vpnstart, vpnstop, cleanup, exit")
		return nil
	case "config":
		fmt.Fprintln(a.stdout, a.config.Summary())
		return nil
	case "start":
		session, err := a.runtimeManager.BeginSession(ctx)
		if err != nil {
			return err
		}
		printSession(a.stdout, session)
		a.runtimeManager.StartHeartbeatLoop(ctx, session.PIN)
		return nil
	case "status":
		session, err := a.runtimeManager.GetSessionStatus(ctx, a.config.SessionPIN)
		if err != nil {
			return err
		}
		printSession(a.stdout, session)
		return nil
	case "pin":
		return a.printSessionPIN()
	case "ping":
		return a.sendHeartbeat(ctx)
	case "stop":
		return a.stopSession(ctx)
	case "scanwifi":
		return a.scanWiFi(ctx)
	case "wifistatus":
		return a.printWiFiStatus(ctx)
	case "connectwifi":
		if len(fields) < 3 {
			return errors.New("connectwifi requires <ssid> <password>")
		}
		return a.connectWiFi(ctx, fields[1], strings.Join(fields[2:], " "))
	case "disconnectwifi":
		return a.disconnectWiFi(ctx)
	case "vpnstatus":
		return a.printVPNStatus(ctx)
	case "vpnstart":
		return a.startVPN(ctx)
	case "vpnstop":
		return a.stopVPN(ctx)
	case "cleanup":
		return a.cleanup(ctx)
	case "exit", "quit":
		a.runtimeManager.StopHeartbeatLoop()
		fmt.Fprintln(a.stdout, "interactive mode exited")
		return io.EOF
	default:
		return fmt.Errorf("unknown interactive command %q", command)
	}
}

func emptyAsUnset(value string) string {
	if value == "" {
		return "<unset>"
	}
	return value
}

func (a *App) startSession(ctx context.Context) error {
	session, err := a.runtimeManager.BeginSession(ctx)
	if err != nil {
		return err
	}

	printSession(a.stdout, session)
	return nil
}

func (a *App) printSessionStatus(ctx context.Context) error {
	session, err := a.runtimeManager.GetSessionStatus(ctx, a.config.SessionPIN)
	if err != nil {
		return err
	}

	printSession(a.stdout, session)
	return nil
}

func (a *App) printSessionPIN() error {
	pin, err := a.runtimeManager.CurrentPIN(a.config.SessionPIN)
	if err != nil {
		return err
	}

	fmt.Fprintln(a.stdout, pin)
	return nil
}

func (a *App) sendHeartbeat(ctx context.Context) error {
	if err := a.runtimeManager.SendHeartbeat(ctx, a.config.SessionPIN); err != nil {
		return err
	}

	fmt.Fprintln(a.stdout, "heartbeat sent")
	return nil
}

func (a *App) stopSession(ctx context.Context) error {
	if err := a.runtimeManager.StopSession(ctx, a.config.SessionPIN); err != nil {
		return err
	}

	fmt.Fprintln(a.stdout, "session ended")
	return nil
}

func printSession(w io.Writer, session backend.SupportSession) {
	fmt.Fprintf(w, "status=%s\n", session.Status)
	fmt.Fprintf(w, "pin=%s\n", session.PIN)
	fmt.Fprintf(w, "ip_address=%s\n", session.IPAddress)
}

func (a *App) handleRuntimeEvent(event agentruntime.Event) {
	switch event.Kind {
	case agentruntime.EventHeartbeatStarted:
		fmt.Fprintf(a.stdout, "automatic heartbeat started (%s)\n", event.Interval)
	case agentruntime.EventHeartbeatFatal:
		fmt.Fprintf(a.stdout, "automatic heartbeat stopped: %v\n", event.Err)
	case agentruntime.EventHeartbeatError:
		fmt.Fprintf(a.stdout, "automatic heartbeat error: %v\n", event.Err)
	case agentruntime.EventHeartbeatStopped:
		fmt.Fprintln(a.stdout, "automatic heartbeat stopped")
	case agentruntime.EventSessionResumed:
		a.logger.Info("service mode resumed persisted session", "pin", event.PIN)
	case agentruntime.EventSessionEnded:
		a.logger.Info("service mode ended active session", "pin", event.PIN)
	case agentruntime.EventWiFiScanCompleted:
		a.logger.Info("wifi scan completed", "count", len(event.Networks))
	case agentruntime.EventWiFiStateChanged:
		a.logger.Info("wifi state changed", "state", event.State)
	case agentruntime.EventVPNStateChanged:
		a.logger.Info("vpn state changed", "state", event.State)
	}
}

func (a *App) scanWiFi(ctx context.Context) error {
	networksFound, err := a.wifiManager.Scan(ctx)
	if err != nil {
		return err
	}

	runtimeNetworks := make([]agentruntime.WiFiNetwork, 0, len(networksFound))
	for _, networkFound := range networksFound {
		runtimeNetworks = append(runtimeNetworks, agentruntime.WiFiNetwork{SSID: networkFound.SSID})
		fmt.Fprintln(a.stdout, networkFound.SSID)
	}
	a.runtimeManager.UpdateWiFiNetworks(runtimeNetworks)
	return nil
}

func (a *App) connectWiFi(ctx context.Context, ssid, password string) error {
	if err := a.wifiManager.Connect(ctx, ssid, password); err != nil {
		return err
	}

	a.runtimeManager.SetWiFiState(agentruntime.BinaryStateConnected)
	a.runtimeManager.SetWiFiStatus(true, true, network.SupportConnectionName)
	fmt.Fprintf(a.stdout, "wifi connected: %s\n", ssid)
	return nil
}

func (a *App) disconnectWiFi(ctx context.Context) error {
	if err := a.wifiManager.Disconnect(ctx); err != nil {
		return err
	}

	a.runtimeManager.SetWiFiState(agentruntime.BinaryStateDisconnected)
	a.runtimeManager.SetWiFiStatus(false, false, "")
	fmt.Fprintln(a.stdout, "wifi disconnected")
	return nil
}

func (a *App) printWiFiStatus(ctx context.Context) error {
	status, err := a.wifiManager.Status(ctx)
	if err != nil {
		return err
	}

	a.runtimeManager.SetWiFiState(agentruntime.BinaryState(status.State))
	a.runtimeManager.SetWiFiStatus(status.AnyActive, status.SupportActive, status.ActiveConnectionName)
	fmt.Fprintf(a.stdout, "wifi_active=%s\n", strconv.FormatBool(status.AnyActive))
	fmt.Fprintf(a.stdout, "support_wifi_active=%s\n", strconv.FormatBool(status.SupportActive))
	fmt.Fprintf(a.stdout, "active_connection=%s\n", emptyAsUnset(status.ActiveConnectionName))
	return nil
}

func (a *App) printVPNStatus(ctx context.Context) error {
	status, err := a.vpnManager.Status(ctx)
	if err != nil {
		return err
	}

	a.runtimeManager.SetVPNState(agentruntime.BinaryState(status.State))
	fmt.Fprintf(a.stdout, "state=%s\n", status.State)
	fmt.Fprintf(a.stdout, "service_active=%s\n", strconv.FormatBool(status.ServiceActive))
	fmt.Fprintf(a.stdout, "interface_present=%s\n", strconv.FormatBool(status.InterfacePresent))
	fmt.Fprintf(a.stdout, "ip_address=%s\n", status.IPAddress)
	fmt.Fprintf(a.stdout, "status_file_present=%s\n", strconv.FormatBool(status.StatusFilePresent))
	return nil
}

func (a *App) startVPN(ctx context.Context) error {
	if err := a.vpnManager.Start(ctx); err != nil {
		return err
	}

	status, err := a.vpnManager.Status(ctx)
	if err != nil {
		return err
	}

	a.runtimeManager.SetVPNState(agentruntime.BinaryState(status.State))
	fmt.Fprintf(a.stdout, "vpn state=%s\n", status.State)
	return nil
}

func (a *App) stopVPN(ctx context.Context) error {
	if err := a.vpnManager.Stop(ctx); err != nil {
		return err
	}

	a.runtimeManager.SetVPNState(agentruntime.BinaryStateDisconnected)
	fmt.Fprintln(a.stdout, "vpn stopped")
	return nil
}

func (a *App) cleanup(ctx context.Context) error {
	if a.cleaner == nil {
		a.cleaner = network.NewCleaner(a.wifiManager, a.vpnManager)
	}

	if err := a.cleaner.Cleanup(ctx); err != nil {
		return err
	}

	a.runtimeManager.SetWiFiState(agentruntime.BinaryStateDisconnected)
	a.runtimeManager.SetVPNState(agentruntime.BinaryStateDisconnected)
	fmt.Fprintln(a.stdout, "cleanup completed")
	return nil
}

func (a *App) syncNetworkState(ctx context.Context) error {
	wifiStatus, err := a.wifiManager.Status(ctx)
	if err != nil {
		return err
	}
	a.runtimeManager.SetWiFiState(agentruntime.BinaryState(wifiStatus.State))
	a.runtimeManager.SetWiFiStatus(wifiStatus.AnyActive, wifiStatus.SupportActive, wifiStatus.ActiveConnectionName)

	vpnStatus, err := a.vpnManager.Status(ctx)
	if err != nil {
		return err
	}
	a.runtimeManager.SetVPNState(agentruntime.BinaryState(vpnStatus.State))
	return nil
}

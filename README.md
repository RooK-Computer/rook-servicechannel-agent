# RooK Servicechannel Agent

This repository hosts the RooK console agent implementation.

## Current State

The repository now contains the backend-facing CLI MVP, the reusable runtime core, the local IPC surface, the WLAN/OpenVPN integration, and a first Debian packaging path for the RooK console agent. The shared system architecture and implementation status are maintained in the `spec/` submodule, while repository-local execution plans live in `plans/`.

## Current Delivery Strategy

The long-term target is a Go-based agent that runs as a local service, owns support-session state, integrates with WLAN and OpenVPN, and later serves a local UI through IPC.

The first MVP is intentionally smaller:

- build an interactive CLI tool first,
- focus on communication with the backend,
- make the support-session lifecycle testable in an integration environment,
- then extend the same runtime toward the full local agent responsibilities.

## Repository Layout

- `spec/` - shared architecture and cross-component implementation documents
- `plans/` - repository-local implementation plans and handoff material
- `AGENTS.md` - instructions for future coding agents
- `cmd/rook-agent` - bootstrap executable entrypoint
- `internal/config` - configuration loading and validation
- `internal/logging` - logging setup
- `internal/app` - current application bootstrap surface
- `internal/backend` - reserved backend adapter boundary
- `internal/runtime` - reserved runtime core boundary
- `internal/ipc` - local Unix socket transport and JSON UI contract
- `internal/network` - local WLAN, OpenVPN, and cleanup adapters

## Key Documents

- `spec/docs/architecture/servicechannel-concept.md`
- `spec/implementation/02-rook-agent-status.md`
- `spec/implementation/10-komponentenuebergreifender-entwicklungsplan.md`
- `plans/README.md`

## Planned Implementation Path

1. Bootstrap the Go project structure and development baseline.
2. Deliver the backend-facing interactive CLI MVP.
3. Extract and harden the runtime core for service mode.
4. Add local IPC for the future console UI.
5. Integrate WLAN, OpenVPN, and cleanup.
6. Add packaging and release-oriented operating work.

## Local Development

Current development commands:

- `make build`
- `make test`
- `make fmt`
- `make run`
- `make package`
- `make tidy`
- `go test ./...`
- `go run ./cmd/rook-agent --interactive --backend-url https://backend.example.test`
- `go run ./cmd/rook-agent service --backend-url https://backend.example.test`
- `go run ./cmd/rook-agent --backend-url https://backend.example.test`
- `go run ./cmd/rook-agent config`
- `go run ./cmd/rook-agent start --backend-url https://backend.example.test`
- `go run ./cmd/rook-agent status --backend-url https://backend.example.test`
- `go run ./cmd/rook-agent pin`
- `go run ./cmd/rook-agent ping --backend-url https://backend.example.test`
- `go run ./cmd/rook-agent stop --backend-url https://backend.example.test`

## Configuration

The backend API endpoint is configurable from the start.

Supported inputs:

- flag: `--backend-url`
- environment: `ROOK_AGENT_BACKEND_URL`

Additional bootstrap configuration:

- flag / environment: `--console-id` / `ROOK_AGENT_CONSOLE_ID`
- flag / environment: `--log-level` / `ROOK_AGENT_LOG_LEVEL`
- flag / environment: `--state-path` / `ROOK_AGENT_STATE_PATH`
- flag / environment: `--socket-path` / `ROOK_AGENT_SOCKET_PATH`
- flag / environment: `--pin` / `ROOK_AGENT_PIN`
- flag / environment: `--ssid` / `ROOK_AGENT_WIFI_SSID`
- flag / environment: `--wifi-password` / `ROOK_AGENT_WIFI_PASSWORD`

Flags override environment variables.

In packaged deployments, the agent service reads its runtime environment from `/etc/default/rook-agent`.
That file uses `KEY=VALUE` lines with optional `#` comments and currently ships:

- `ROOK_AGENT_BACKEND_URL`
- `ROOK_AGENT_LOG_LEVEL`
- `ROOK_AGENT_STATE_PATH`
- `ROOK_AGENT_SOCKET_PATH`
- optional `ROOK_AGENT_CONSOLE_ID`

## Runtime and CLI Commands

The current agent binary exposes a service-style runtime path, an IPC-backed interactive console for that service, and a session-centric direct CLI surface against the backend REST API.

Service-oriented execution:

- no explicit command: run service mode and wait for shutdown
- `service`: run service mode explicitly

In service mode, the agent resumes a locally persisted active session, continues heartbeats in the background, and attempts to end the session cleanly during graceful shutdown.

Service mode also starts the local Unix domain socket IPC server for a later console UI.

Primary mode:

- `--interactive` opens a prompt-driven IPC client for the running local service

Interactive mode requirements:

- the local service must already be running
- the Unix socket at `ROOK_AGENT_SOCKET_PATH` must be reachable
- if no service/socket is available, interactive mode exits with a clear error instead of silently falling back to local execution

Available commands inside the prompt:

- `help` - show supported commands
- `config` - print the effective configuration
- `start` - ask the running service to begin a support session
- `status` - query the current support status from the running service
- `pin` - print the active session PIN
- `ping` - ask the running service to send an extra manual heartbeat immediately
- `stop` - ask the running service to end the active session
- `scanwifi` - ask the running service to list visible WiFi SSIDs
- `wifistatus` - report whether any WiFi connection is active and whether it is the RooK support WiFi profile
- `connectwifi <ssid> <password>` - ask the running service to create and activate the temporary RooK support WiFi profile
- `disconnectwifi` - ask the running service to remove the temporary RooK support WiFi profile
- `vpnstatus` - print the effective OpenVPN status reported by the running service
- `vpnstart` - ask the running service to start the OpenVPN client service
- `vpnstop` - ask the running service to stop the OpenVPN client service
- `cleanup` - ask the running service to remove temporary WiFi/VPN support artifacts
- `exit` - leave the interactive shell

Direct subcommands remain available as a secondary interface:

- `config` - print the effective configuration
- `start` - begin a support session and persist the active session locally
- `status` - query the current backend session status
- `pin` - print the active session PIN from local state or `--pin`
- `ping` - send a manual heartbeat
- `stop` - end the active session and clear local state
- `scanwifi` - list visible WiFi SSIDs
- `wifistatus` - report whether any WiFi connection is active and whether it is the RooK support WiFi profile
- `connectwifi --ssid <name> --wifi-password <password>` - connect the temporary support WiFi profile
- `disconnectwifi` - remove the temporary support WiFi profile
- `vpnstatus` - print the effective OpenVPN status
- `vpnstart` - start the OpenVPN client service
- `vpnstop` - stop the OpenVPN client service
- `cleanup` - stop VPN and remove temporary support WiFi state

If `--pin` is not provided, `status`, `pin`, `ping`, and `stop` use the locally persisted session state file.

In interactive mode, asynchronous service events such as `SupportStateChanged`, `PinAssigned`, WLAN state changes, VPN state changes, and service-side error events are shown in the prompt output.

The direct session commands still execute locally against backend/runtime boundaries. The interactive prompt now exercises the service path over the local IPC socket.

## Local IPC Contract

The current IPC surface is available in service mode over a Unix domain socket.

Defaults:

- socket path: `ROOK_AGENT_SOCKET_PATH` or the configured default under the user config directory
- transport: Unix domain socket
- message format: streamed JSON request/response plus asynchronous event messages

For packaged UI integrations, the supported way to discover the socket is:

1. read `/etc/default/rook-agent`
2. resolve `ROOK_AGENT_SOCKET_PATH`
3. connect to that Unix socket path

If the packaged config keeps its shipped default, that path is `/run/rook-agent/agent.sock`.

Currently implemented request actions:

- `GetStatus`
- `Ping`
- `ScanWifi`
- `ConnectWifi`
- `DisconnectWifi`
- `VpnStatus`
- `VpnStart`
- `VpnStop`
- `Cleanup`
- `StartSupport`
- `StopSupport`
- `GetPin`

The `GetStatus` payload currently distinguishes between:

- `wifiState` for the RooK-managed support WiFi state,
- `anyWifiActive` for a general active WiFi connection on the host,
- `supportWifiActive` for the RooK support WiFi profile specifically,
- `activeWifiConnection` for the currently active WiFi connection name when available.

Currently implemented asynchronous events:

- `WifiScanCompleted`
- `WifiConnectionStateChanged`
- `VpnStateChanged`
- `SupportStateChanged`
- `PinAssigned`
- `ErrorRaised`

## Debian Packaging

The repository now contains a first Debian packaging path for the current delivery slice.

Artifacts and assets:

- `make package` builds `build/packages/rook-agent_0.0.0-1~dev_amd64.deb`
- `packaging/nfpm.yaml` defines the package metadata and installed files
- `packaging/systemd/rook-agent.service` provides the packaged service unit
- `packaging/default/rook-agent` provides packaged runtime configuration defaults

Installed paths in the package:

- binary: `/usr/bin/rook-agent`
- service unit: `/lib/systemd/system/rook-agent.service`
- environment file: `/etc/default/rook-agent`

Packaged runtime defaults:

- backend URL: `ROOK_AGENT_BACKEND_URL` in `/etc/default/rook-agent`
- log level: `ROOK_AGENT_LOG_LEVEL`
- state path: `/var/lib/rook-agent/session.json`
- socket path: `/run/rook-agent/agent.sock`

Typical packaged service flow:

1. Build the package:
   - `make package`
2. Install it on the target Debian system:
   - `sudo dpkg -i build/packages/rook-agent_0.0.0-1~dev_amd64.deb`
3. Adjust `/etc/default/rook-agent` for the target backend and console environment.
4. Enable and start the service:
   - `sudo systemctl enable --now rook-agent.service`
5. Optionally connect with the interactive service console:
   - `rook-agent --interactive`

## Operations and Diagnostics

Useful packaged-operation commands:

- `systemctl status rook-agent.service`
- `journalctl -u rook-agent.service`
- `rook-agent config`
- `rook-agent wifistatus`
- `rook-agent vpnstatus`

First-line operator checks:

1. confirm the backend URL in `/etc/default/rook-agent`
2. inspect the service state in `systemctl status`
3. inspect logs in `journalctl -u rook-agent.service`
4. confirm runtime paths under `/var/lib/rook-agent` and `/run/rook-agent`

## Manual MVP Integration Flow

1. Start the service:
   - `go run ./cmd/rook-agent service --backend-url https://backend.example.test`
2. Connect with the interactive service console:
   - `go run ./cmd/rook-agent --interactive --backend-url https://backend.example.test`
3. Inside the prompt, start the session:
   - `start`
4. Read the assigned PIN:
   - `pin`
5. Inspect current session state while the service heartbeat is running:
   - `status`
6. End the session cleanly:
   - `stop`
7. Leave the prompt:
   - `exit`

The current implementation now includes WLAN management, OpenVPN control/observation, cleanup handling, and a first installable Debian package path. Further release hardening remains a later follow-up.

## Notes

The `spec/` directory is a Git submodule because the architecture and implementation status are shared across multiple repositories in the wider RooK servicechannel project.

# RooK Servicechannel Agent

This repository hosts the RooK console agent implementation.

## Current State

The repository now contains the first backend-facing CLI MVP for the RooK console agent. The shared system architecture and implementation status are maintained in the `spec/` submodule, while repository-local execution plans live in `plans/`.

## Current Delivery Strategy

The long-term target is a Go-based agent that runs as a local service, owns support-session state, integrates with WLAN and OpenVPN, and later serves a local UI through IPC.

The first MVP is intentionally smaller:

- build an interactive CLI tool first,
- focus on communication with the backend,
- make the support-session lifecycle testable in an integration environment,
- defer WLAN and VPN work to later phases.

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
5. Integrate WLAN, OpenVPN, cleanup, and packaging.

## Local Development

Current development commands:

- `make build`
- `make test`
- `make fmt`
- `make run`
- `make tidy`
- `go test ./...`
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
- flag / environment: `--pin` / `ROOK_AGENT_PIN`

Flags override environment variables.

## CLI MVP Commands

The current CLI MVP is session-centric and talks directly to the backend REST API.

Available commands:

- `config` - print the effective configuration
- `start` - begin a support session and persist the active session locally
- `status` - query the current backend session status
- `pin` - print the active session PIN from local state or `--pin`
- `ping` - send a manual heartbeat
- `stop` - end the active session and clear local state

If `--pin` is not provided, `status`, `pin`, `ping`, and `stop` use the locally persisted session state file.

## Manual MVP Integration Flow

1. Start the agent session:
   - `go run ./cmd/rook-agent start --backend-url https://backend.example.test`
2. Read the assigned PIN:
   - `go run ./cmd/rook-agent pin`
3. Inspect session status:
   - `go run ./cmd/rook-agent status --backend-url https://backend.example.test`
4. Send a heartbeat:
   - `go run ./cmd/rook-agent ping --backend-url https://backend.example.test`
5. End the session:
   - `go run ./cmd/rook-agent stop --backend-url https://backend.example.test`

This MVP intentionally focuses on the backend session lifecycle only. WLAN, VPN runtime control, and local UI IPC remain future phases.

## Notes

The `spec/` directory is a Git submodule because the architecture and implementation status are shared across multiple repositories in the wider RooK servicechannel project.

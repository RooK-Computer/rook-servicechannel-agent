# RooK Servicechannel Agent

This repository hosts the RooK console agent implementation.

## Current State

The repository is currently in planning/bootstrap mode. The shared system architecture and implementation status are maintained in the `spec/` submodule, while repository-local execution plans live in `plans/`.

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

## Notes

The `spec/` directory is a Git submodule because the architecture and implementation status are shared across multiple repositories in the wider RooK servicechannel project.

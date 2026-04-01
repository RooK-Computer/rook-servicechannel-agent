# RooK Agent Implementation Plans

## Purpose

This folder contains the repository-local implementation plans for `rook-servicechannel-agent`.

The source architecture and cross-component status live in the shared `spec/` submodule. These plans translate that shared concept into executable work packages for this repository, with enough structure for later agents to continue the work without rebuilding context from scratch.

## Source Documents

- `spec/docs/architecture/servicechannel-concept.md`
- `spec/implementation/02-rook-agent-status.md`
- `spec/implementation/10-komponentenuebergreifender-entwicklungsplan.md`

## Planning Assumption

The long-term target remains the Go-based agent running as a `systemd` service. However, the first MVP for this repository is intentionally narrower:

- build an interactive CLI tool first,
- focus on agent-to-backend communication,
- make the session lifecycle testable in an integration environment,
- defer WLAN configuration and VPN status querying to later phases.

This means the repository should be planned as an evolution path:

1. bootstrap the agent codebase,
2. ship a backend-facing CLI MVP,
3. extract a durable runtime core,
4. add service mode, IPC, WLAN, VPN, and packaging on top.

## Progress Dashboard

| Plan | Scope | Status |
| --- | --- | --- |
| `01-foundation-and-bootstrap.md` | Repository bootstrap and engineering baseline | Planned |
| `02-cli-mvp-backend-lifecycle.md` | Interactive CLI MVP for backend session flow | Planned |
| `03-runtime-core-and-service-mode.md` | State model, background runtime, daemon evolution | Planned |
| `04-local-ipc-and-ui-contract.md` | Unix socket IPC and later console UI integration | Planned |
| `05-network-and-vpn-integration.md` | WLAN, OpenVPN, cleanup, reboot recovery | Planned |
| `06-packaging-operations-and-release.md` | Packaging, observability, deployment, release gates | Planned |

## Handoff Rules

When an agent starts implementation work, it should:

1. read this file,
2. read the relevant detailed plan file,
3. update `spec/implementation/02-rook-agent-status.md`,
4. update the status fields in the touched plan files,
5. keep the CLI-first MVP boundary explicit until a later plan phase is started.

## Review Gate Policy

Every plan phase ends with a mandatory review stop.

That means:

1. complete the phase scope,
2. update the relevant plan file and status documents,
3. stop further implementation work,
4. wait for user review before starting the next plan.

This repository intentionally uses review checkpoints between plans so the user can inspect the result before work continues.

## Deferred Concept Work

The shared architecture currently describes the agent primarily as a daemon with WLAN and VPN responsibilities. A follow-up spec update is required to document the CLI-first MVP as an intentional early delivery mode and to explain how it evolves into the full agent runtime.

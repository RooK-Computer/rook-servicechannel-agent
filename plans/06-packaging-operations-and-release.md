# Plan 06 - Packaging, Operations, and Release

Status: Planned

## Goal

Make the repository deployable, supportable, and releasable on the target console environment.

## Scope

- package the agent with `nfpm`,
- define service files and installation layout,
- document runtime configuration,
- add operational logging and diagnostics guidance,
- define release readiness checks.

## Workstreams

| Workstream | Description | Status |
| --- | --- | --- |
| Packaging layout | Define file locations, config paths, and package contents | Planned |
| Service assets | Add `systemd` unit files and install hooks | Planned |
| Runtime config docs | Document required environment and secrets handling | Planned |
| Observability | Define logs, debug commands, and failure triage surface | Planned |
| Release checklist | Establish validation gates for packaged delivery | Planned |
| Upgrade behavior | Plan package upgrades and local state migration | Planned |

## Dependencies

- Enough implementation from Plans 03 through 05 to package something meaningful.

## Deliverables

- package configuration,
- deployable service assets,
- operations runbook content in the README or adjacent docs,
- release checklist for future delivery work.

## Exit Criteria

- The agent can be installed and started in a repeatable way.
- Operators know where to configure, inspect, and troubleshoot it.
- Packaging decisions stay aligned with the shared architecture and spec repository.

## Handoff Notes

If packaging work starts before WLAN and VPN are implemented, package the CLI MVP and runtime core first, but document that the package is intentionally incomplete relative to the final architecture.

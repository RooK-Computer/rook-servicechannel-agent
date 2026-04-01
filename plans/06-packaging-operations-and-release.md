# Plan 06 - Packaging, Operations, and Release

Status: Planned

## Goal

Make the repository deployable, supportable, and releasable on the target console environment.

## Scope

- package the agent with `nfpm`,
- define service files and installation layout,
- produce an installable Debian package for the implemented delivery slice,
- document runtime configuration,
- ensure the backend API endpoint remains configurable in packaged deployments,
- add operational logging and diagnostics guidance,
- define release readiness checks.

## Workstreams

| Workstream | Description | Status |
| --- | --- | --- |
| Packaging layout | Define file locations, config paths, and package contents for an installable Debian package | Planned |
| Service assets | Add `systemd` unit files and install hooks | Planned |
| Runtime config docs | Document required environment, configurable backend API endpoint, and secrets handling | Planned |
| Observability | Define logs, debug commands, and failure triage surface | Planned |
| Release checklist | Establish validation gates for packaged delivery | Planned |
| Upgrade behavior | Plan package upgrades and local state migration | Planned |

## Dependencies

- Enough implementation from Plans 03 through 05 to package something meaningful.

## Deliverables

- package configuration,
- installable Debian package output,
- deployable service assets,
- operations runbook content in the README or adjacent docs,
- release checklist for future delivery work.

## Exit Criteria

- The agent can be installed and started in a repeatable way.
- The packaged installation exposes a supported way to configure the backend API endpoint.
- Operators know where to configure, inspect, and troubleshoot it.
- Packaging decisions stay aligned with the shared architecture and spec repository.

## Review Gate

When this plan is implemented, stop after packaging validation and wait for user review before any further follow-up work.

## Handoff Notes

If packaging work starts before WLAN and VPN are implemented, package the CLI MVP and runtime core first, but document that the package is intentionally incomplete relative to the final architecture.

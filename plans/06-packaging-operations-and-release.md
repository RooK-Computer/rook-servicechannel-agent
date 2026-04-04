# Plan 06 - Packaging, Operations, and Release

Status: Approved

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
| Packaging layout | Define file locations, config paths, and package contents for an installable Debian package | Done |
| Service assets | Add `systemd` unit files and install hooks | Done |
| Runtime config docs | Document required environment, configurable backend API endpoint, and secrets handling | Done |
| Observability | Define logs, debug commands, and failure triage surface | Done |
| Release checklist | Establish validation gates for packaged delivery | Done |
| Upgrade behavior | Plan package upgrades and local state migration | Done |

## Dependencies

- Enough implementation from Plans 03 through 05 to package something meaningful.

## Deliverables

- package configuration,
- installable Debian package output,
- deployable service assets,
- operations runbook content in the README or adjacent docs,
- release checklist for future delivery work.

## Implementation Notes

Implemented artifacts:

- `packaging/nfpm.yaml` for Debian packaging via `nfpm`,
- `packaging/systemd/rook-agent.service` for the packaged service-mode runtime,
- `packaging/default/rook-agent` for packaged runtime environment defaults,
- maintainer scripts under `packaging/scripts/`,
- `make package` as the packaging entrypoint,
- README updates for packaged installation, configuration, diagnostics, and service-backed interactive usage.
- shared spec updates that document `/etc/default/rook-agent` as the packaged agent configuration file and explain all shipped parameters.
- debug-level observability now includes inbound IPC messages plus outbound IPC/backend result payloads, with obvious secret fields redacted before they reach operator logs.

Implementation choices in this phase:

- the first packaged format is Debian only,
- the packaged service runs the already existing `rook-agent service` path instead of introducing a separate daemon binary,
- packaged runtime paths are fixed to `/var/lib/rook-agent/session.json` and `/run/rook-agent/agent.sock`,
- backend endpoint configuration remains environment-driven through `/etc/default/rook-agent`,
- the packaged config file format follows systemd `EnvironmentFile` semantics with `KEY=VALUE` entries and comments starting with `#`,
- package installation reloads `systemd`, but service enable/start remains an explicit operator step,
- the interactive mode now acts as an IPC client for the running service and fails clearly if the packaged service/socket is unavailable.

## Verification

Validation completed with:

- `make test`
- `make build`
- `make package`
- `dpkg-deb -c build/packages/rook-agent_0.0.0-1~dev_amd64.deb`
- `dpkg-deb -I build/packages/rook-agent_0.0.0-1~dev_amd64.deb`
- interactive prompt tests against a running in-process service in `internal/app/app_test.go`

The resulting state after this phase:

- the agent can be built into an installable Debian package,
- the package ships the binary, service unit, and packaged environment file,
- operators have a documented way to configure backend URL, inspect logs, and start the service,
- `ROOK_AGENT_LOG_LEVEL=debug` now exposes request/response-level activity for service troubleshooting without writing WiFi passwords in cleartext,
- release gates and conservative upgrade expectations are now explicit in repository documentation.

## Release Checklist

Before calling a Plan 06 delivery ready, check at least:

1. `make test` passes.
2. `make build` produces the expected binary.
3. `make package` produces the Debian artifact.
4. The package contains `/usr/bin/rook-agent`, `/lib/systemd/system/rook-agent.service`, and `/etc/default/rook-agent`.
5. `/etc/default/rook-agent` still exposes a supported way to configure the backend endpoint.
6. README and status documents match the packaged runtime paths and operator flow.

## Upgrade Behavior

Current package upgrade expectations:

- `/etc/default/rook-agent` is treated as operator-managed config and must not be silently overwritten,
- local runtime state remains under `/var/lib/rook-agent`,
- package upgrades should preserve state unless a future migration step is explicitly documented,
- service enable/start remains an operator choice after install or upgrade.

## Exit Criteria

- The agent can be installed and started in a repeatable way.
- The packaged installation exposes a supported way to configure the backend API endpoint.
- Operators know where to configure, inspect, and troubleshoot it.
- Packaging decisions stay aligned with the shared architecture and spec repository.

## Review Gate

When this plan is implemented, stop after packaging validation and wait for user review before any further follow-up work.

## Handoff Notes

If packaging work starts before WLAN and VPN are implemented, package the CLI MVP and runtime core first, but document that the package is intentionally incomplete relative to the final architecture.

At the review stop for this phase:

- stop before any new follow-up phase,
- review whether the first Debian package shape and operator flow are acceptable,
- treat stricter hardening of the packaged service as follow-up work rather than silently broadening Plan 06.

## Review Outcome

Plan 06 has been reviewed and approved.

Confirmed outcome from the review:

- the first Debian package shape is accepted as the current delivery baseline,
- the service-backed interactive console is accepted as part of the packaged operating model,
- further packaging hardening, operational polish, or architecture alignment should be treated as explicit follow-up work.

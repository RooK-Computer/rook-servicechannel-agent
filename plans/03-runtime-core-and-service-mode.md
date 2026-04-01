# Plan 03 - Runtime Core and Service Mode

Status: Planned

## Goal

Evolve the CLI MVP into the long-lived agent core described in the shared architecture without discarding the earlier work.

## Scope

- introduce a domain state model for support sessions,
- add a background runtime that owns heartbeats and lifecycle transitions,
- support clean startup and shutdown,
- add service-mode execution alongside the CLI mode,
- preserve the CLI as a diagnostics and operator tool.

## Workstreams

| Workstream | Description | Status |
| --- | --- | --- |
| Domain state model | Model support lifecycle states and transitions | Planned |
| Runtime loop | Add background processing for heartbeats and expiry checks | Planned |
| Persistence | Store enough local state for restart and cleanup behavior | Planned |
| Service mode | Add a long-running process mode intended for `systemd` | Planned |
| Shutdown handling | Ensure stop, timeout, and reboot paths are explicit | Planned |
| Diagnostics CLI | Reuse the runtime core for debug and inspection commands | Planned |

## Dependencies

- Plan 02 completed or at least stable enough that backend behavior is known.

## Deliverables

- reusable runtime core,
- service-mode executable path,
- state transition tests,
- documented migration path from CLI MVP to daemon behavior.

## Exit Criteria

- The agent can own session state without a human manually driving every heartbeat.
- The CLI remains available as an operational surface instead of being thrown away.
- Later IPC, WLAN, and VPN features plug into the same runtime core.

## Review Gate

When this plan is implemented, stop after runtime-core and service-mode validation and wait for user review before starting Plan 04.

## Handoff Notes

Treat the CLI MVP as the first adapter around the runtime core, not as throwaway code. This keeps the repository aligned with both the MVP request and the long-term architecture.

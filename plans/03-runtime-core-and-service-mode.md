# Plan 03 - Runtime Core and Service Mode

Status: Approved

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
| Domain state model | Model support lifecycle states and transitions | Done |
| Runtime loop | Add background processing for heartbeats and expiry checks | Done |
| Persistence | Store enough local state for restart and cleanup behavior | Done |
| Service mode | Add a long-running process mode intended for `systemd` | Done |
| Shutdown handling | Ensure stop, timeout, and reboot paths are explicit | Done |
| Diagnostics CLI | Reuse the runtime core for debug and inspection commands | Done |

## Dependencies

- Plan 02 completed or at least stable enough that backend behavior is known.

## Deliverables

- reusable runtime core,
- service-mode executable path,
- state transition tests,
- documented migration path from CLI MVP to daemon behavior.

## Implementation Notes

Implemented artifacts:

- reusable runtime manager in `internal/runtime/manager.go`,
- runtime-owned heartbeat loop with explicit event reporting for start, stop, retryable errors, and fatal backend failures,
- service-mode execution path that resumes a locally persisted session and ends it cleanly during graceful shutdown,
- CLI refactoring so direct commands and interactive mode both reuse the same runtime core instead of owning their own backend/session logic,
- runtime tests for lifecycle, background heartbeat, and service-mode resume/shutdown behavior.

Implementation choices in this phase:

- the default command path now behaves as service-mode execution,
- the explicit `service` command is available alongside the existing CLI commands,
- persisted session state remains the source of truth for service resume and session-scoped CLI commands,
- graceful service shutdown ends the active backend session and clears local state.

## Verification

Validation completed with:

- `make fmt`
- `make test`

The resulting state after this phase:

- the agent owns heartbeat scheduling outside the interactive prompt,
- CLI mode remains usable as an operator and diagnostics surface,
- later IPC, WLAN, and VPN work can target the runtime core instead of command-specific logic.

## Exit Criteria

- The agent can own session state without a human manually driving every heartbeat.
- The CLI remains available as an operational surface instead of being thrown away.
- Later IPC, WLAN, and VPN features plug into the same runtime core.

## Review Gate

When this plan is implemented, stop after runtime-core and service-mode validation and wait for user review before starting Plan 04.

## Handoff Notes

Treat the CLI MVP as the first adapter around the runtime core, not as throwaway code. This keeps the repository aligned with both the MVP request and the long-term architecture.

At the review stop for this phase:

- begin Plan 04 next, not Plan 05,
- keep WLAN and VPN explicitly in repository scope,
- wire IPC onto the runtime core before adding the local network/system adapters.

## Review Outcome

Plan 03 has been reviewed and approved.

Confirmed outcome from the review:

- the runtime core and service mode are accepted as the new baseline,
- Plan 04 is now the active implementation phase,
- WLAN and VPN remain explicitly in scope, but only after the IPC layer is in place.

# Plan 02 - CLI MVP Backend Lifecycle

Status: Planned

## Goal

Deliver the first usable MVP as an interactive CLI tool that exercises the backend session lifecycle end to end.

## Intentional MVP Boundary

This phase deliberately does **not** implement:

- WLAN scanning or connection management,
- OpenVPN start/stop automation,
- VPN status querying,
- local UI socket IPC,
- `systemd` service behavior.

Those remain part of the long-term architecture, but they are deferred so the agent and backend can be integrated in a test environment first.

## Recommended Shape

Implement the first slice as an operator-driven CLI with a small interactive command loop and reusable subcommands. The exact UX can evolve, but it should let developers and testers:

- load agent configuration,
- identify the console,
- start a support session,
- fetch or display the assigned PIN,
- inspect current remote session state,
- send a heartbeat manually or on a short timer,
- stop the session cleanly,
- inspect last known local state for debugging.

## Workstreams

| Workstream | Description | Status |
| --- | --- | --- |
| Backend contract alignment | Define request and response payloads needed for the MVP | Planned |
| Console identity | Decide how the CLI identifies the console to the backend | Planned |
| Session start | Implement the session creation flow | Planned |
| Session status | Implement status retrieval and local rendering | Planned |
| PIN handling | Display the current PIN and validity metadata | Planned |
| Heartbeats | Support manual and optional timed heartbeat sending | Planned |
| Session stop | Implement clean session termination | Planned |
| Local state file | Persist the active session reference for continuity between commands | Planned |
| Error UX | Make backend failures explicit and operator-readable | Planned |

## Dependencies

- Plan 01 bootstrap completed or sufficiently advanced.
- Backend must provide a minimal API contract or a mocked equivalent.

## Deliverables

- interactive CLI MVP,
- backend client abstraction,
- local session persistence,
- test coverage for success and failure flows,
- documentation for manual integration testing.

## Verification

- A tester can start a session from the CLI and observe the backend acknowledging it.
- A tester can retrieve the current PIN and session status.
- A tester can send at least one heartbeat and end the session.
- Failure cases such as invalid configuration or backend errors are visible in the terminal.

## Related Spec Work

This phase should trigger a shared spec update that explains:

- why the repository starts with a CLI-first agent slice,
- which responsibilities are temporarily deferred,
- how the CLI MVP maps onto the later full agent.

## Handoff Notes

Do not hardwire backend payload handling directly into the command layer. Put backend communication behind an internal package so the same client can be reused by future service mode and IPC handlers.

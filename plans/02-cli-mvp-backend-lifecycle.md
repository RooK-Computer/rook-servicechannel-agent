# Plan 02 - CLI MVP Backend Lifecycle

Status: Implemented, review pending

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

Implement the first slice as an operator-driven CLI with a prompt-driven interactive mode and reusable direct subcommands. The exact UX can evolve, but it should let developers and testers:

- load agent configuration,
- identify the console,
- start a support session,
- fetch or display the assigned PIN,
- inspect current remote session state,
- send a heartbeat manually or on a short timer,
- stop the session cleanly,
- inspect last known local state for debugging.

The CLI must use the configured backend API endpoint rather than assuming a fixed server URL.

The intended primary UX is:

- start the agent in an explicit interactive mode such as `--interactive`,
- open a dedicated prompt,
- trigger lifecycle commands from that prompt,
- automatically keep the backend session alive after `start`.

## Workstreams

| Workstream | Description | Status |
| --- | --- | --- |
| Backend contract alignment | Define request and response payloads needed for the MVP | Done |
| Console identity | Decide how the CLI identifies the console to the backend | Deferred by contract |
| Session start | Implement the session creation flow | Done |
| Session status | Implement status retrieval and local rendering | Done |
| PIN handling | Display the current PIN and validity metadata | Done |
| Heartbeats | Support manual and optional timed heartbeat sending | Done |
| Session stop | Implement clean session termination | Done |
| Local state file | Persist the active session reference for continuity between commands | Done |
| Error UX | Make backend failures explicit and operator-readable | Done |
| Interactive prompt | Provide a prompt-driven operator mode | Done |
| Automatic heartbeat loop | Keep the active session alive after `start` | Done |

## Execution Order

1. Backend contract alignment
2. Backend client implementation
3. CLI command structure and operator UX
4. Local session persistence
5. Interactive prompt and automatic heartbeat loop
6. Tests, docs, and review preparation

## Detailed Implementation Slices

### Slice 1 - Backend contract alignment

- define internal request and response types for:
  - session start,
  - session status,
  - heartbeat,
  - PIN retrieval,
  - session stop
- identify any open contract assumptions in the shared spec
- document the assumptions close to the implementation work

### Slice 2 - Backend client

- add a backend client package under the reserved backend boundary
- implement HTTP transport and response decoding
- keep error handling explicit and operator-readable
- ensure all requests use the configured backend base URL

### Slice 3 - CLI surface

- choose a command layout that supports both direct invocation and operator-driven usage
- support at minimum:
  - status/config inspection,
  - session start,
  - session status,
  - PIN display,
  - heartbeat trigger,
  - session stop

### Slice 4 - Interactive prompt runtime

- add an explicit interactive mode such as `--interactive`
- open a dedicated prompt with the supported lifecycle commands
- keep prompt handling separate from backend transport details
- allow the operator to inspect current session state from the prompt

### Slice 5 - Local persistence and heartbeat loop

- persist the active session reference locally
- start an automatic heartbeat loop after `start`
- stop the heartbeat loop cleanly on `stop`, exit, or fatal session error
- make heartbeat failures visible instead of silently swallowing them

### Slice 6 - Validation and docs

- add tests for success and failure flows
- use a fake or mock backend path where direct backend integration is not available
- update README and status documents
- prepare the phase for the mandatory review stop

## Dependencies

- Plan 01 bootstrap completed or sufficiently advanced.
- Backend must provide a minimal API contract or a mocked equivalent.

## Deliverables

- interactive CLI MVP,
- prompt-driven interactive mode,
- backend client abstraction,
- local session persistence,
- automatic heartbeat behavior after session start,
- test coverage for success and failure flows,
- documentation for manual integration testing.

## Implementation Notes

Implemented artifacts:

- backend contract DTOs and operation constants in `internal/backend/contract.go`,
- HTTP backend client in `internal/backend/client.go`,
- interactive prompt mode plus direct CLI command handling for `config`, `start`, `status`, `pin`, `ping`, and `stop`,
- local session state persistence in `internal/sessionstate`,
- automatic heartbeat loop started from the interactive `start` command,
- tests for contract validation, HTTP client behavior, session persistence, and CLI lifecycle flows,
- README usage and manual integration instructions.

Implementation choices retained for the MVP:

- the backend contract follows `spec/openapi/02-agent-backend-rest.openapi.yaml`,
- `beginsession` uses an empty request body,
- no separate backend PIN endpoint is used; the PIN is taken from the start and status responses,
- session-scoped CLI commands default to the locally persisted state file unless `--pin` is provided,
- direct subcommands may remain available even after the interactive prompt is added.

## Dependencies in Practice

- Plan 01 is approved.
- The shared architecture defines the operations conceptually, but concrete payloads must still be derived during implementation.
- If the backend repository is not yet concrete enough, a temporary fake backend contract may be required for tests.

## Verification

- A tester can start a session from the CLI and observe the backend acknowledging it.
- A tester can retrieve the current PIN and session status.
- A tester can send at least one heartbeat and end the session.
- Failure cases such as invalid configuration or backend errors are visible in the terminal.
- The same binary can target different backend environments through configuration alone.

Validation completed with:

- `make fmt`
- `make test`
- interactive prompt smoke test (`help`, `exit`)

## Related Spec Work

This phase should trigger a shared spec update that explains:

- why the repository starts with a CLI-first agent slice,
- which responsibilities are temporarily deferred,
- how the CLI MVP maps onto the later full agent.

## Handoff Notes

Do not hardwire backend payload handling directly into the command layer. Put backend communication behind an internal package so the same client can be reused by future service mode and IPC handlers.

## Review Gate

When this plan is implemented, stop after the prompt-driven CLI MVP and its validation are complete. Do not continue into runtime-core work before the user has reviewed the phase.

## Handoff Notes for Execution

- Update `spec/implementation/02-rook-agent-status.md` as soon as Slice 1 starts.
- Treat backend contract alignment as the only valid starting point for implementation.
- Do not let persistence or CLI UX choices leak transport details into the domain-facing code.

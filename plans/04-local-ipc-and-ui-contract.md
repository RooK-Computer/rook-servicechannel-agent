# Plan 04 - Local IPC and UI Contract

Status: Planned

## Goal

Add the local control surface needed for a later console UI while keeping state ownership in the agent.

## Scope

- implement the Unix domain socket transport described in the shared spec,
- define JSON request and event payloads,
- connect socket handlers to the runtime core,
- support reconnect-friendly status retrieval,
- document the contract well enough for the UI repository to integrate against it.

## Workstreams

| Workstream | Description | Status |
| --- | --- | --- |
| Socket transport | Create the Unix socket server and lifecycle handling | Planned |
| Request contract | Define request and response types for supported actions | Planned |
| Event stream | Emit asynchronous state-change events | Planned |
| Auth and permissions | Decide local access and filesystem permissions for the socket | Planned |
| Reconnect behavior | Ensure UI restarts can recover current state | Planned |
| Integration fixtures | Add examples or test fixtures for UI-side consumers | Planned |

## Dependencies

- Runtime core from Plan 03.
- Shared agreement with the UI team on contract details.

## Deliverables

- working local IPC server,
- documented request and event schema,
- tests for reconnect and event sequencing,
- updated status docs describing IPC readiness.

## Exit Criteria

- A separate UI process can request current agent state and receive updates.
- The UI does not need to implement its own support logic.

## Handoff Notes

Keep request types aligned with the shared architecture, but revisit the command list once WLAN and VPN integration are implemented so the contract reflects real capabilities.

# Plan 04 - Local IPC and UI Contract

Status: Approved

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
| Socket transport | Create the Unix socket server and lifecycle handling | Done |
| Request contract | Define request and response types for supported actions | Done |
| Event stream | Emit asynchronous state-change events | Done |
| Auth and permissions | Decide local access and filesystem permissions for the socket | Done |
| Reconnect behavior | Ensure UI restarts can recover current state | Done |
| Integration fixtures | Add examples or test fixtures for UI-side consumers | Done |

## Dependencies

- Runtime core from Plan 03.
- Shared agreement with the UI team on contract details.

## Deliverables

- working local IPC server,
- documented request and event schema,
- tests for reconnect and event sequencing,
- updated status docs describing IPC readiness.

## Implementation Notes

Implemented artifacts:

- local Unix domain socket server in `internal/ipc/server.go`,
- JSON contract types for requests, responses, and asynchronous events in `internal/ipc/contract.go`,
- aligned OpenAPI message contract in `spec/openapi/01-ui-agent-local-ipc.openapi.yaml`,
- service-mode integration so the IPC server starts alongside the runtime core,
- runtime snapshot access and event subscription support so IPC stays attached to the existing state owner,
- tests for start/stop event sequencing and reconnect-friendly status retrieval.
- shared spec updates that explain how the UI resolves the agent socket path in packaged deployments through `/etc/default/rook-agent` and `ROOK_AGENT_SOCKET_PATH`.

Implementation choices in this phase:

- the socket transport uses a single long-lived Unix socket connection per client with JSON messages streamed over the same connection,
- the concrete transport semantics are `AF_UNIX` / `SOCK_STREAM` with one JSON object per line, using newline as the message delimiter rather than EOF or packet boundaries,
- packaged UI integrations resolve the socket path from the shared agent environment file `/etc/default/rook-agent` instead of probing for candidate paths,
- supported request actions in this first contract slice are `GetStatus`, `StartSupport`, `StopSupport`, and `GetPin`,
- asynchronous events currently include `SupportStateChanged`, `PinAssigned`, and `ErrorRaised`,
- socket directories created by the agent use `0755` permissions and the socket node itself is set to `0666` so non-root local clients can reach the root-run agent over IPC.

## Verification

Validation completed with:

- `make fmt`
- `make test`

The resulting state after this phase:

- a separate local UI process can query the current agent status over IPC,
- the UI can trigger support-session start and stop without owning backend or lifecycle logic,
- reconnecting clients can read the current agent state from the runtime-backed local snapshot.

## Exit Criteria

- A separate UI process can request current agent state and receive updates.
- The UI does not need to implement its own support logic.

## Review Gate

When this plan is implemented, stop after IPC validation and wait for user review before starting Plan 05.

## Handoff Notes

Keep request types aligned with the shared architecture, but revisit the command list once WLAN and VPN integration are implemented so the contract reflects real capabilities.

At the review stop for this phase:

- begin Plan 05 next, not Plan 06,
- keep the IPC contract extensible for later WLAN and VPN commands,
- avoid moving state ownership out of `internal/runtime`; extend the existing runtime core instead.

## Review Outcome

Plan 04 has been reviewed and approved.

Confirmed outcome from the review:

- the local IPC layer is accepted as the new UI integration baseline,
- the next active implementation phase is Plan 05 for WLAN, OpenVPN, and cleanup,
- the existing IPC contract should now be extended rather than replaced.
- a later spec clarification aligned the non-normative message and event catalog with the already implemented `ScanWifi` contract: empty request payload, `WiFiScanPayload` response, and matching `WifiScanCompleted` event payload.

# Plan 05 - Network and VPN Integration

Status: Approved

## Goal

Implement the deferred local system responsibilities so the repository reaches the full agent role from the shared architecture.

## Scope

- WLAN scanning and connection via `nmcli`,
- temporary support-network lifecycle,
- OpenVPN integration,
- VPN status detection,
- cleanup after session end or reboot,
- explicit error paths for local system failures.

## Workstreams

| Workstream | Description | Status |
| --- | --- | --- |
| WLAN discovery | Scan and normalize available wireless networks | Done |
| WLAN connection | Connect, disconnect, and report connection state | Done |
| Temporary profile cleanup | Remove RooK support profiles after session end or boot | Done |
| OpenVPN control | Start and stop the VPN client service | Done |
| VPN observation | Read service and interface signals to determine effective status | Done |
| Runtime integration | Connect network transitions to the support session state model | Done |
| Failure recovery | Handle partial failures and cleanup consistently | Done |

## Dependencies

- Runtime core from Plan 03.
- OpenVPN infrastructure readiness from the shared program plan.
- Access to target console environments for integration testing.

## Deliverables

- WLAN adapter,
- OpenVPN adapter,
- cleanup logic,
- integration tests or scripted validation on target systems,
- updated IPC and CLI surfaces for network-aware commands.

## Implementation Notes

Implemented artifacts:

- WLAN adapter in `internal/network/network.go` built around `nmcli`,
- OpenVPN control and status observation in the same package using `systemctl`, the `rookvpn` interface, and `/var/log/rook-openvpn/client-status.log`,
- cleanup helper that stops the OpenVPN client and removes the RooK support WiFi profile,
- runtime extensions for observable WiFi/VPN state and boot-aware recovery,
- IPC extensions for `ScanWifi`, `ConnectWifi`, `DisconnectWifi`, `WifiScanCompleted`, `WifiConnectionStateChanged`, and `VpnStateChanged`,
- direct CLI commands for WiFi scan/status/connect/disconnect, VPN status/start/stop, and cleanup.

Implementation choices in this phase:

- the temporary WiFi connection is managed under the fixed NetworkManager profile name `rook-support-wifi`,
- WLAN status observation now distinguishes between "any WiFi connection is active" and "the RooK support WiFi profile is active",
- VPN connectivity is considered effectively connected only when the service is active and the `rookvpn` interface has an IPv4 address,
- reboot recovery clears locally persisted support-session state when the stored boot ID no longer matches the running system,
- service-mode startup performs cleanup only when no active session is being resumed or when reboot recovery has invalidated stale state.

## Verification

Validation completed with:

- `make fmt`
- `make test`
- `make build`

The resulting state after this phase:

- the agent can scan, connect, and disconnect temporary support WiFi,
- the agent can report whether any WiFi connection is active and whether that connection is the RooK support WiFi profile,
- the agent can start, stop, and observe the OpenVPN client from concrete local signals,
- the IPC and CLI surfaces now expose the first network-aware controls,
- reboot and cleanup handling remove stale local support artifacts instead of blindly resuming them.

## Exit Criteria

- The agent can create and remove support connectivity without manual operator intervention.
- VPN state can be queried from actual local signals, not guessed.
- Reboot or crash recovery does not leave RooK-owned temporary network artifacts behind.

## Review Gate

When this plan is implemented, stop after network and VPN validation and wait for user review before starting Plan 06.

## Handoff Notes

The shared spec already references `rook-openvpn-client.service`, the `rookvpn` TUN interface, and `/var/log/rook-openvpn/client-status.log`. Reuse those paths instead of inventing alternatives unless the shared spec is updated first.

At the review stop for this phase:

- begin Plan 06 next, not additional feature work,
- keep Debian packaging as the next repo-local phase,
- treat future refinements to the network contract as follow-up hardening, not as a reason to split state ownership away from the runtime core.

## Review Outcome

Plan 05 has been reviewed and approved.

Confirmed outcome from the review:

- the network and cleanup integration is accepted as the current agent baseline,
- the WiFi status follow-up for CLI and GUI is included in the approved phase result,
- the next active implementation phase is Plan 06 for packaging, operations, and release preparation.

# Plan 05 - Network and VPN Integration

Status: Planned

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
| WLAN discovery | Scan and normalize available wireless networks | Planned |
| WLAN connection | Connect, disconnect, and report connection state | Planned |
| Temporary profile cleanup | Remove RooK support profiles after session end or boot | Planned |
| OpenVPN control | Start and stop the VPN client service | Planned |
| VPN observation | Read service and interface signals to determine effective status | Planned |
| Runtime integration | Connect network transitions to the support session state model | Planned |
| Failure recovery | Handle partial failures and cleanup consistently | Planned |

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

## Exit Criteria

- The agent can create and remove support connectivity without manual operator intervention.
- VPN state can be queried from actual local signals, not guessed.
- Reboot or crash recovery does not leave RooK-owned temporary network artifacts behind.

## Handoff Notes

The shared spec already references `rook-openvpn-client.service`, the `rookvpn` TUN interface, and `/var/log/rook-openvpn/client-status.log`. Reuse those paths instead of inventing alternatives unless the shared spec is updated first.

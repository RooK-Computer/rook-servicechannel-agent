# AGENTS.md

## Repository Purpose

This repository contains the implementation of the RooK console agent. At the moment, the shared `spec/` submodule is the main source of truth for architecture and cross-component status, while the repository-local planning and implementation work lives here.

## Read First

Before making changes, read:

1. `plans/README.md`
2. the relevant file in `plans/`
3. `spec/implementation/02-rook-agent-status.md`
4. `spec/docs/architecture/servicechannel-concept.md` when the change touches architecture assumptions

## Working Rules

- Communicate with the user in German.
- Keep root repository documentation in English unless there is a strong reason not to.
- The shared `spec/` submodule currently uses German and should stay consistent with its existing language.
- Do not treat the current CLI-first MVP as a contradiction of the long-term daemon architecture. Treat it as the first delivery slice.
- Keep the domain core reusable across CLI mode, future service mode, and future IPC handlers.
- Treat a configurable backend API endpoint as a core requirement, not a convenience.
- Keep Debian packaging in scope for the repository roadmap and implementation decisions.

## Mandatory Documentation Updates

Whenever you make meaningful progress in this repository, update:

- `spec/implementation/02-rook-agent-status.md`
- the touched file(s) in `plans/`
- `README.md` if repository usage, scope, or setup changes

If you change architecture assumptions beyond repository-local execution details, plan a follow-up update to the shared concept docs in `spec/`.

## MVP Boundary

The first MVP is an interactive CLI tool for backend communication and session lifecycle testing.

Until a later phase explicitly starts, treat these topics as out of scope for the MVP:

- WLAN configuration
- OpenVPN automation
- VPN status querying
- local UI IPC
- final `systemd` service behavior

## Handoff Expectations

- Keep status fields current in plan files.
- Record deferred work explicitly instead of silently omitting it.
- Prefer small, composable internal packages over monolithic command code.
- Preserve a usable CLI surface even after service mode is introduced.
- After completing any plan phase, stop and wait for user review before moving to the next phase.

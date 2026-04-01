# Plan 01 - Foundation and Bootstrap

Status: Implemented, review pending

## Goal

Create the initial project structure so implementation can start without revisiting basic repository decisions.

## Why This Comes First

The repository currently has no application code. Later phases need a stable place for the CLI surface, runtime core, transport adapters, configuration, and tests.

## Scope

- initialize the Go module and toolchain baseline,
- define the project layout,
- choose the command entrypoint structure,
- establish configuration loading,
- make the backend API endpoint explicitly configurable from day one,
- establish logging conventions,
- create a lightweight testing baseline,
- document local development commands in the root README.

## Workstreams

| Workstream | Description | Status |
| --- | --- | --- |
| Module bootstrap | Initialize `go.mod`, dependency policy, and versioning approach | Done |
| Project layout | Create directories for `cmd/`, `internal/`, config, and tests | Done |
| Config baseline | Define how backend API base URL, console identity, and auth material are loaded and overridden | Done |
| Logging baseline | Pick structured logging and error-reporting conventions | Done |
| CLI shell skeleton | Create the initial command tree or interactive shell entrypoint | Done |
| Test baseline | Add unit test structure and one smoke test target | Done |
| Developer docs | Document setup and common commands | Done |

## Dependencies

- No code dependency.
- Assumes the backend contract will be refined in parallel or immediately afterward.

## Deliverables

- runnable `go` project skeleton,
- one executable entrypoint,
- baseline config contract,
- baseline tests,
- repository docs updated to explain local development.

## Exit Criteria

- `go test ./...` passes for the bootstrap skeleton,
- the repository has a clear place for CLI, runtime core, and adapters,
- the backend API endpoint can be configured without code changes,
- the next agent can begin backend work without making layout decisions first.

## Implementation Notes

Implemented bootstrap artifacts:

- `go.mod` initialized for the repository,
- executable entrypoint in `cmd/rook-agent`,
- `Makefile` for standard build, test, format, run, tidy, and clean workflows,
- bootstrap app surface in `internal/app`,
- config loading and validation in `internal/config`,
- structured logging setup in `internal/logging`,
- reserved package boundaries for `internal/backend` and `internal/runtime`,
- tests for configuration precedence and application smoke execution,
- README development and configuration guidance.

Validation completed with:

- `gofmt -w ...`
- `go test ./...`

## Review Gate

When this plan is implemented, stop after bootstrap validation and hand the repository to the user for review before starting Plan 02.

## Handoff Notes

Keep the domain core separate from the CLI surface from day one. The CLI MVP should use the same internal core that later service mode and IPC mode will reuse.

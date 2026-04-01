# Plan 01 - Foundation and Bootstrap

Status: Planned

## Goal

Create the initial project structure so implementation can start without revisiting basic repository decisions.

## Why This Comes First

The repository currently has no application code. Later phases need a stable place for the CLI surface, runtime core, transport adapters, configuration, and tests.

## Scope

- initialize the Go module and toolchain baseline,
- define the project layout,
- choose the command entrypoint structure,
- establish configuration loading,
- establish logging conventions,
- create a lightweight testing baseline,
- document local development commands in the root README.

## Workstreams

| Workstream | Description | Status |
| --- | --- | --- |
| Module bootstrap | Initialize `go.mod`, dependency policy, and versioning approach | Planned |
| Project layout | Create directories for `cmd/`, `internal/`, config, and tests | Planned |
| Config baseline | Define how backend URL, console identity, and auth material are loaded | Planned |
| Logging baseline | Pick structured logging and error-reporting conventions | Planned |
| CLI shell skeleton | Create the initial command tree or interactive shell entrypoint | Planned |
| Test baseline | Add unit test structure and one smoke test target | Planned |
| Developer docs | Document setup and common commands | Planned |

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
- the next agent can begin backend work without making layout decisions first.

## Handoff Notes

Keep the domain core separate from the CLI surface from day one. The CLI MVP should use the same internal core that later service mode and IPC mode will reuse.

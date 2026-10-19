# Build Explanation — Cycle 1963

## Build Binding
- Cycle: 1963
- Base SHA: b6e7c8f2ec60ec02095d7878820b01430a2aef7f

## Summary
The new `evolve rebuild` command builds the plane binary from HEAD to `go/bin/evolve` outside the automated boot refresh cycle.

## Rationale
Manual binary builds previously required `make -C go build`. The new command unifies the make build invocation, guards active run leases, and prints binary SHAs.

## Changed Areas
- `go/cmd/evolve/cmd_rebuild.go` — implements the `evolve rebuild` subcommand with lease checks, SHA reporting, and failure tail output.
- `go/cmd/evolve/cmd_rebuild_test.go` — provides unit tests for lease refusal, failure tail retention, and SHA output.
- `go/cmd/evolve/registry.go` — registers the `rebuild` subcommand in the command table.
- `go/cmd/evolve/main.go` — adds `rebuild` syntax to the usage documentation.
- `docs/operations/workspace-layout.md` — replaces raw make command guidance with `evolve rebuild`.
- `docs/operations/runtime-reference.md` — documents `evolve rebuild` behavior, lease checks, failure tail output, and shared build logic.

## Design Decisions
The `rebuild` verb calls `defaultBootRefreshRebuild`. Both boot refresh and manual rebuild share one build function. Run lease checks query `runlease.LiveRuns` to stop builds during active cycles. Build failure displays the output tail through `stderrTail`.

## Verification
ACS predicates in `go/acs/cycle1963/predicates_test.go` verify all eight criteria. Unit tests in `go/cmd/evolve/cmd_rebuild_test.go` verify flag parsing, lease guards, and tail formatting.

## Compatibility
Existing boot refresh logic remains unchanged. The new command adds CLI capability without breaking existing workflows.

## Limitations
The rebuild verb targets the local host environment only.

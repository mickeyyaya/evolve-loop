# Build Explanation — Cycle 1843

## Build Binding
- Cycle: 1843
- Base SHA: b7d31e54fb7e448bf35740f7264308bef0261756

## Summary
Refuse mid-wave route-console and route-lane operations while a loop lane is live, preserving inbox file contents without modification.

## Rationale
Routing an inbox item while a loop lane holds an active run lease under `.evolve/runs/` mutates tracked repository state unexpectedly, which can disrupt concurrent lane execution and fail audit tree-state verification. Mid-wave refusal guards against concurrent mutation.

## Changed Areas
- `go/cmd/evolve/cmd_inbox_route_console.go` — checks `refusedMidWave("route-console", stderr)` after argument validation to refuse execution with exit code 1 when a loop lane is active.
- `go/cmd/evolve/cmd_inbox_route_lane.go` — checks `refusedMidWave("route-lane", stderr)` after argument validation to refuse execution with exit code 1 when a loop lane is active.

## Design Decisions
Reused existing `refusedMidWave` helper from `cmd_inbox_curate.go` rather than introducing duplicate lease query logic. Placed the lease check strictly after CLI argument validation and usage checks so that usage errors continue to exit with code 10 before lease checks. Both verbs exit with code 1 upon refusal, output clear diagnostics naming the active lane PID and cycle, and leave target inbox files byte-identical without rewriting.

## Verification
Unit tests in `go/cmd/evolve/cmd_inbox_route_midwave_test.go` and ACS predicates in `go/acs/cycle1843/predicates_test.go` verify that live leases trigger refusal with exit code 1 and byte-identical retention, stale leases proceed normally, and usage errors retain exit code 10 precedence.

## Compatibility
The CLI arguments, option semantics, and behavior when no live loop lease exists remain completely unchanged. Stale leases are ignored and do not refuse routing.

## Limitations
This cycle specifically covers mid-wave refusal for the `route-console` and `route-lane` subcommands; earlier curation verbs were previously addressed.

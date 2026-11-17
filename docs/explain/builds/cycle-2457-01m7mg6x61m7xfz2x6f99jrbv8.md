# Build Explanation — Cycle 2457

## Build Binding
- Cycle: 2457
- Base SHA: ed3bf7d91654154ae0d35b697bd017017f0cf911

## Summary
The supply census now splits the old `console_route` class into `console_route_fact` and `console_route_no_fact`.

## Rationale
A route with a protected declared file has a checked reason. A route with no protected file has none. The console must see the two counts apart (plan K2).

## Changed Areas
- `go/internal/orchestrator/supply.go` — the two new classes, and the class of a route item comes from its declared paths.
- `go/internal/orchestrator/orchestrator.go` — the signal description names the seven classes.
- `go/internal/wavemeasure/wavemeasure.go` — the measure carries both counts in place of one.
- `go/cmd/evolve/cmd_wave_measures.go` — the console total sums both counts.
- `go/cmd/evolve/cmd_wave_supply.go` — the class column is wide enough for the new names.
- `docs/architecture/wave-orchestrator.md` — the class table lists the two classes.
- `docs/operations/wave-operations.md` — the class list names the two classes.

## Design Decisions
The old class name is removed. Each route item counts once, so `Supply.Total` stays correct and the sum of the two counts equals the old count.

## Verification
The three ACS predicates of cycle 2457 pass. The orchestrator, wavemeasure and wave command tests pass.

## Compatibility
The JSON key `console_route` no longer exists in the census counts and in the measures.

## Limitations
Old records in `supply.json` keep the key `console_route`, and readers must treat it as absent.

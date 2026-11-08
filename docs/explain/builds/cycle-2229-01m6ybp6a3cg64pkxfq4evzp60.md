# Build Explanation — Cycle 2229

## Build Binding
- Cycle: 2229
- Base SHA: aa661665f6b210bac310a925fb9a80143b37beb9

## Summary
A failed retry ladder now marks its state and emits the `LADDER_REDESIGN` signal once, which names the redesign rung of ADR-0126 as the route. A second end of the ladder goes to the operator.

## Rationale
The operator ruled on 2026-11-06 (ADR-0164 ruling 1) that the redesign runs first. The route reuses the existing ladder state and the signal registry, so no new package or flag is needed.

## Changed Areas
- `go/internal/core/retry_ladder.go` — `askLadder` calls `routeToRedesign` at R3 or a spec stop, emits `LADDER_REDESIGN` and marks the state.
- `go/internal/cyclestate/state.go` — `LadderState` gains the `Redesign` mark, so a second end is detectable.
- `go/internal/core/retry_ladder_test.go` — unit tests for the end, the spec stop, the early boundary and the second end.
- `go/internal/core/retry_ladder_redesign_runcycle_test.go` — tests through the real `RunCycle` entry.
- `go/internal/core/ladder_redesign_apicover_test.go` — names `CodeLadderRedesign` in a real assertion.
- `go/acs/cycle2229/predicates_test.go` — four predicates for the route and the record.
- `docs/architecture/adr/0146-a-defect-that-survives-two-builder-rounds-is-fixed-from-the-finders-spec-and-verified-by-another.md` — item 9 now names the redesign route.
- `docs/architecture/signal-codes.md` — lists `LADDER_REDESIGN`.
- `.evolve/evals/ladder-end-routes-to-the-redesign-rung.md` — graders point at the cycle2229 package.

## Design Decisions
One `Redesign` string on the ladder state bounds the route to one use. The rejected option was a new counter field, because the string also records the failed check.

## Verification
The ACS suite for the cycle reports 267 green and 0 red. The `Redesign` tests in `internal/core` pass, and the predicates pass with the `acs` tag.

## Compatibility
Existing ladder rungs and signal codes keep their behavior. The new field is additive.

## Limitations
No production code yet dispatches the convergence redesign rung from the mark. It is deferred to a later cycle.

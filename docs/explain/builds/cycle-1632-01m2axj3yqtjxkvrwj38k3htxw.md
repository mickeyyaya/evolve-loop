# Build Explanation — Cycle 1632

## Build Binding
- Cycle: 1632
- Base SHA: 6ca7b70239849696ce8b03ee91d369c2ada0d114

## Summary
Typed carryover and previous-verdict fields are now bounded before they reach phase prompts or shadow diagnostics.

## Rationale
Applying one cap at `CycleInputs` construction keeps the representation consistent across dispatch and comparison without creating a new raw carryover prompt writer.

## Changed Areas
- `.evolve/inbox/2026-07-07T09-30-00Z-tokenopt-handoff-digests.json` — removes the active inbox copy after this cycle consumed its partial scope, preventing the same work from being dispatched again as unconsumed.
- `.evolve/inbox/consumed/2026-07-07T09-30-00Z-tokenopt-handoff-digests.json` — preserves the consumed original inbox record so its request and lifecycle history remain auditable after it leaves the active queue.
- `.evolve/inbox/2026-09-12T23-00-00Z-tokenopt-handoff-digests-per-edge-remainder.json` — tracks the original inbox item's unmet per-edge configuration and remaining prompt-consumer work so this partial build does not claim full closure.
- `go/internal/phaseio/cycleinputs.go` — defines the shared UTF-8-safe field cap and applies it at the typed input boundary.
- `go/internal/core/phaseio_shadow.go` — compares the same bounded legacy representation so shadow diagnostics neither report false drift nor expose uncapped text.
- `go/internal/phaseio/digest.go` — gives each upstream section a deterministic sub-budget so one oversized scalar cannot evict the other sealed handoff sections.
- `go/internal/phases/tdd/tdd.go` — injects that bounded upstream digest into the active typed-input TDD prompt using `phaseio`'s package-default cap, avoiding a divergent call-site limit.

## Design Decisions
The existing typed DTO remains the single input surface. Only the two unbounded free-text fields are capped; established identity and routing fields retain byte-identical behavior.

## Verification
Cycle-owned unit and ACS tests exercise boundary values, multibyte input, production dispatch, cap-equivalent comparisons, genuine drift, and the no-raw-writer invariant.

## Compatibility
At-or-under-cap values are unchanged, and no public DTO field or getter is removed.

## Limitations
The cap is fixed at 16 KiB and does not introduce per-phase or operator configuration.

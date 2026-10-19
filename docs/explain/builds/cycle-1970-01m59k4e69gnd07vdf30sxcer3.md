# Build Explanation — Cycle 1970

## Build Binding
- Cycle: 1970
- Base SHA: f1ea865b040b04e86c6f67752ce6e62d7346feb2

## Summary
`Decide` in `internal/handoff/exit` now returns a `blocked` exit when an injected prover proves the blocker claims of the blocked results.

## Rationale
Cycles 1946 and 1950 spent three correction rounds on asks no lane could meet. The blocker-exit seam in `core` already proves such claims. The exit package takes the seam as a `Prover` function, so it needs no second protected-path matcher and no import of `core` or `guards`.

## Changed Areas
- `go/internal/handoff/exit/exit.go` — adds `Blocked`, `Claim`, `Proof`, `Prover`, `Policy.Prover`, `Result.Claim`, `Decision.Blocked` and `Decision.Paths`, and the proof step before the cap test.
- `go/internal/handoff/exit/apicover_named_test.go` — names the new exports in a real assertion.
- `docs/architecture/packages/internal-handoff.md` — documents the blocked exit.

## Design Decisions
The exit is `blocked` only when the prover reports `Blocked` and every unmet required ask is named by a proven claim, so a claim cannot excuse another unmet ask. `blocked` wins over the cap and over no gain. `pass` still wins over a stale claim, and the prover is not called then. Blocked ids follow request order and paths are sorted.

## Verification
`blocker_test.go` and the ACS predicates `TestC1970_001` to `004` run the cycle 1950 conflict and the selfcheck protected path through the real `core.NewBlockerProver`.

## Compatibility
`Policy` and `Result` gain optional fields. A zero `Policy` behaves as before.

## Limitations
No production code calls `Decide` yet. The console wiring of the prover is a later step of plan H4/H5.

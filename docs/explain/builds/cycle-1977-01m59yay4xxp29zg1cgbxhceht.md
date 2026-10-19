# Build Explanation — Cycle 1977

## Build Binding
- Cycle: 1977
- Base SHA: e66b2a66792670e650922b3f637176446143d611

## Summary
The retry ladder gets its rung decider, `retryladder.Next`. It maps a classified failed try, the rung of the try and the earlier fingerprints to one decision: retry, raise or stop.

## Rationale
The spec fixes the rungs (R0, R1, R2) and the stop rules. One pure function keeps them in one place and lets a table test and a property test prove the bounds. The function reuses `convergence.TierEffort` and `convergence.HeadroomTable`, so the package carries no second tier table.

## Changed Areas
- `go/internal/retryladder/decide.go` — adds `Next` and its types, the signals and the default bounds.
- `go/internal/retryladder/apicover_named_test.go` — names every new exported symbol in a real assertion.
- `docs/architecture/packages/internal-retryladder.md` — records the decider rules and the missing production caller.

## Design Decisions
R1 keeps the base tier and steps the effort by one level, at most `max_effort`. When no step exists, R1 repeats the base seat in a fresh session. R2 climbs exactly one tier, at most `max_tier`. The first family of the chain with headroom at that tier supplies the model. Rejected: skipping to the nearest tier that has headroom, because the spec says one step up. Rejected: a no-headroom signal when a bound blocks the climb, because the signal means a missing seat, not a policy limit. An empty fingerprint never counts as repeated.

## Verification
`go test -count=1 ./internal/retryladder/` passes, with the table test, the property test over 3000 random bounds and the rule tests. `go test -count=1 -tags acs ./acs/cycle1977/` passes.

## Compatibility
No existing exported symbol changes. `Classify` and `Fingerprint` stay as they were.

## Limitations
`Next` has no production caller yet. A later ladder component must wire it into the loop.

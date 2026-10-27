# Build Explanation — Cycle 2059

## Build Binding
- Cycle: 2059
- Base SHA: f8269cb929be8f9053d677959f8dc2d4d57e981e

## Summary
The retry ladder decider now has a third raised rung, R3. A failed R2 always hands over to R3. R1 skips R2 when its fingerprint repeats or when no tier climb exists. After R3 the ladder stops.

## Rationale
Cycles 1972 and 1983 each failed the audit after two repair rounds. ADR-0146 items 8 and 9 add a last rung with a fresh builder on a second family. The change reuses the existing bounded helpers, so no new setting exists.

## Changed Areas
- `go/internal/retryladder/decide.go` — adds `R3`, the R1 skip rule, the R2 to R3 hand-over and the R3 stop rule. The new `builderSeat` picks the family of the R3 builder.
- `docs/architecture/packages/internal-retryladder.md` — states the new rung rules and the limit of three raised tries.
- `go/internal/retryladder/decide_r3_test.go` — new tests: replay of cycles 1972 and 1983, the skips, the family choice and the last rung.
- `go/internal/retryladder/decide_test.go` — table rows and the property bound follow the new rungs.
- `go/internal/retryladder/decide_amplify_test.go` — two tests move the stop-as-spec rule from R2 to R3.
- `go/internal/retryladder/apicover_named_test.go` — names `R3` in a real assertion.
- `go/internal/core/retry_ladder_test.go` — proves the correction loop dispatches R3 through the production caller.
- `go/internal/core/retry_ladder_paths_test.go` — the resumed correction under enforce now dispatches three tries, because R1 without headroom goes to R3.
- `go/acs/cycle2059/predicates_test.go` — predicates for the five criteria and the caller proof.
- `.evolve/evals/finder-fix-ownership-f2-rung-r3-decider.md` — new eval with six score caps.
- `.evolve/evals/retry-ladder-l2-decider.md` — three stale criteria follow the new rungs.

## Design Decisions
R1 and R2 keep their meaning, because pinned tests depend on it. R3 keeps the base tier, so the builder stays below the auditor tier. The family comes from the chain order, so the choice is deterministic.

## Verification
`go test -count=1` passes for `internal/retryladder` and `internal/core`. The cycle 2059 predicates pass. The ACS suite reports green=269 red=0.

## Compatibility
Rungs R0 to R2 keep their targets. Two behaviors change: R2 no longer stops, and R1 without headroom no longer stops.

## Limitations
The numbering of ADR-0142 is not changed. The ladder allows one more raised try than before.

# Build Explanation — Cycle 1971

## Build Binding
- Cycle: 1971
- Base SHA: eed2fc4dc73077cf7ce3d750f941292ead65883f

## Summary
The TDD phase now corrects narrative comments in its own predicate files at `evolve phase verify tdd`, and the build comment floor skips the cycle's `go/acs/cycle<N>/` files.

## Rationale
The builder may not edit the predicate files, so a comment there could never be fixed by the phase that the build floor blamed. Moving the check to the TDD exit puts the correction with the file's owner. This unblocks the `comment_floor` flip to enforce.

## Changed Areas
- `go/internal/core/comment_floor.go` — adds `withoutTddOwnedPredicates`, which drops this cycle's predicate directory from the build scan, and `TddCommentFloorFailures`, which lists the added comment lines of named files.
- `go/internal/reachabilityprobe/frozenpins.go` — adds `HandoffTestFiles`; `FrozenTestFiles` now shares one handoff reader with it.
- `go/internal/cli/phasecmd/phase_verify.go` — adds the `tdd_comment_floor` violation to the TDD verify, built from the `testFiles` of the handoff.
- `agents/evolve-tdd-engineer.md` — tells the TDD persona that the exit now fails on narrative comments.
- `go/internal/core/comment_floor_tdd_test.go` — covers the build-floor exemption and its limits.
- `go/internal/core/comment_floor_tddexit_test.go` — names `TddCommentFloorFailures` and covers shadow, enforce and off.
- `go/internal/reachabilityprobe/handoff_testfiles_test.go` — names `HandoffTestFiles` for frozen and unfrozen handoffs.
- `go/internal/cli/phasecmd/phase_verify_tddcomment_test.go` — covers the TDD exit end to end.
- `go/acs/cycle1971/predicates_test.go` — the four acceptance predicates of the cycle.

## Design Decisions
The TDD floor judges only the files in the handoff `testFiles`, so it never blames files the builder owns. It fails at shadow and enforce, and stays silent at off, so TDD corrects its files before the build floor flips. A bound cycle exempts only its own directory; another cycle's predicate comments still count. An unbound cycle (cycle 0) exempts any `go/acs/cycle<digits>/` directory and still counts a non-numeric one.

## Verification
The `core`, `phasecmd`, `reachabilityprobe` and `commentaudit` packages pass with `-count=1`. The `cycle1971` ACS predicates pass.

## Compatibility
No flag or policy key changes. The build floor result is unchanged for cycles that add no predicate comments.

## Limitations
This change does not flip `comment_floor` to enforce; that is a separate item.

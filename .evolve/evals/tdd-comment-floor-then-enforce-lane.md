---
score_cap:
  - criterion: "The TDD exit rejects a narrative comment in the cycle's predicate file and accepts the corrected file"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1971_001 ./acs/cycle1971"
  - criterion: "The build floor ignores the cycle's TDD-owned predicate comments and still names the build's own comment"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestCommentFloorFailures_(IgnoresTheCyclesTddOwnedPredicateFile|StillNamesTheBuildsOwnCommentBesideTddOwnedOnes|StillCountsAnotherCyclesPredicateFile)' ./internal/core"
  - criterion: "An unbound cycle (Cycle == 0) exempts any go/acs/cycle<digits>/ predicate dir at the build floor and still names the build's own comment"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestCommentFloorFailures_UnboundCycle' ./internal/core && go test -tags acs -count=1 -run TestC1971_005 ./acs/cycle1971"
---

# Eval: TDD comment floor and build-floor exemption

> Pins the lane part of tdd-comment-floor-then-enforce (comment round 12, PR #749). The build handoff floor counted the comments in go/acs/cycle<N>/predicates_test.go, which the builder may not edit, so comment_floor could not move to enforce. TDD now corrects its own comments at `evolve phase verify tdd`; the build floor skips this cycle's TDD-owned predicate files.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| tdd-exit-floor | A narrative comment in the predicate file fails the TDD exit | 6/10 | `go test -tags acs -run TestC1971_001 ./acs/cycle1971` |
| build-floor-exemption | TDD-owned comments leave the build floor green; the build's own comment is still named | 7/10 | `go test -run TestCommentFloorFailures ./internal/core` |
| unbound-cycle | Cycle 0 exempts numbered predicate dirs, counts `cyclex`, keeps the build comment | 7/10 | `go test -run TestCommentFloorFailures_UnboundCycle ./internal/core` |

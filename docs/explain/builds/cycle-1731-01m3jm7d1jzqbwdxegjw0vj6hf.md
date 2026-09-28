# Build Explanation — Cycle 1731

## Build Binding
- Cycle: 1731
- Base SHA: 82162ffe113be535a4806e1eaedc859a2bf402a3

## Summary
Shrinks the eight `go/internal/failurelog` and `go/internal/router` functions listed as
`sizeratchet` offenders — `PruneByClassification`, `PruneExpired`, `PruneExpiredCarryoverTodos`,
`Record`, `ClampPlanModelRouting`, `ClampPlanToFloorWith`, `Digest`, `shouldRun` — to at or
under the 50-line limit and removes their entries from `go/internal/sizeratchet/offenders.json`.
No public API, request/response shape, or control-flow order changed; every function keeps its
exact original signature and behavior.

## Rationale
Each offender repeated the same shape: read-parse-loop-write for the three prune-style functions
and Record, or a sequence of independent per-entry checks for the two router clamp functions,
Digest, and shouldRun. Extracting the repeated or independent sub-steps into small, unexported
helper functions removes duplication (all three `failurelog` prune functions shared an
identical read/parse/write skeleton) and lets each remaining function read as a short list of
named steps, without changing what any step does or when it runs.

## Changed Areas
- `go/internal/failurelog/prune.go` — factors the shared read-state/write-result skeleton out of
  `PruneExpired` and `PruneByClassification` into new unexported `readStateArray` and
  `writePruneResult` helpers; both functions now call the same two helpers with different keys/
  predicates instead of repeating the skeleton.
- `go/internal/failurelog/prune_carryover.go` — `PruneExpiredCarryoverTodos` now reuses
  `readStateArray`/`writePruneResult` from `prune.go` (same package) instead of repeating its own
  copy of the read/parse/write skeleton.
- `go/internal/failurelog/record.go` — splits `Record` into `loadRecordState` (read+parse
  state.json), `resolveRecordSummary` (the explicit-override/report-path/fallback summary
  resolution), and `appendFailedApproach` (FIFO-trim append + `lastCycleNumber` advance); `Record`
  itself is now a straight-line call of the three helpers plus the atomic write.
- `go/internal/router/model_routing_clamp.go` — splits `ClampPlanModelRouting`'s per-entry body
  into `clampToTierEnvelope`, `clampToProfilePin`, and `clampToCatalog`, each returning
  `(Clamp, bool)` for "did this rule fire"; the loop tries them in the original order and
  `continue`s on the first that fires, preserving the original short-circuit semantics.
- `go/internal/router/floor.go` — extracts `withEvaluatorFloor` (the floor-phase re-assertion),
  `promoteUnifiedCommitmentTier` (the unified-commitment deep-tier loop), and `forcePhase` (the
  former `force` closure, now a named function taking `*[]Clamp`) out of `ClampPlanToFloorWith`.
- `go/internal/router/digest.go` — splits `Digest`'s four near-identical phase blocks into
  `digestScout`, `digestTriagePhase`, `digestBuildPhase`, and `digestAuditPhase`, each taking
  `(workspace string, sig *RoutingSignals)`; `Digest` now calls the four helpers behind the
  same `done[phase]` guards plus the unchanged failure-sentinel fold loop.
- `go/internal/router/router.go` — splits `shouldRun` into `mandatoryPhaseRun` (the
  unconditional/conditional-mandatory pin) and `shouldRunFromPlan` (the Advisory-stage,
  plan-driven branch); `shouldRun` itself now sequences the two helpers plus the unchanged
  trigger-driven fallback switch.
- `go/internal/sizeratchet/offenders.json` — drops the eight now-conforming entries.

## Design Decisions
Every extraction is a pure mechanical split: no helper changes an existing condition, reorders an
existing check, or introduces a new one. Where a function had a shared skeleton with a sibling in
the same package (the three `failurelog` prune functions), the skeleton was factored into one
helper used by all three rather than duplicated a third time — the smallest change that also
removes the pre-existing duplication instead of adding a fourth copy. All extracted helpers are
unexported; none add a new public seam, so no caller-proof or apicover obligations apply.

## Verification
- `cd go && go test -count=1 ./internal/failurelog/... ./internal/router/...` — unmodified
  suites pass, confirming behavior is unchanged.
- `cd go && go test -tags acs -run TestC1731_00[12] -count=1 -v ./acs/cycle1731/...` — the
  eval's own size and offenders.json graders pass.
- `cd go && go test -tags acs -run TestC1731_00[34] -count=1 -v ./acs/cycle1731/...` — the
  behavior-pin grader (re-running the same failurelog/router suites) and the explanation-count grader pass.
- `./go/bin/evolve acs suite --cycle 1731` — verdict=PASS, green=173 red=0 skip=53 total=226.

## Compatibility
No exported signature, JSON shape, or CLI-visible behavior changed.

## Limitations
This change does not touch the two other functions in `prune_carryover.go`
(`BackfillLegacyCarryoverExpiry`, `IncrementCarryoverUnpicked`) that share the same read-state
pattern but were not size offenders and were out of this task's scope.

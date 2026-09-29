---
score_cap:
  - criterion: "applyCarryoverDecisions in go/cmd/evolve/cmd_carryover.go is <=50 lines (sizeratchet ceiling; was 57)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1753_001_ApplyCarryoverDecisionsWithinSizeRatchetLimit ./acs/cycle1753/..."
  - criterion: "The unlocked pre-read fast path is extracted into a carryoverNoOpFastPath helper that applyCarryoverDecisions actually calls, not merely deleted or inlined elsewhere"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1753_003_ApplyCarryoverDecisionsCallsExtractedFastPathHelper ./acs/cycle1753/..."
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists cmd/evolve.applyCarryoverDecisions (the ratchet only tightens)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1753_005_OffendersJSONNoLongerListsApplyCarryoverDecisions ./acs/cycle1753/..."
  - criterion: "carryoverNoOpFastPath has its own unit tests covering the fast-path hit (no matching removal id) and the divergent branch (a matching id present falls through to the locked update)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestCarryoverNoOpFastPath_' ./cmd/evolve/..."
  - criterion: "Existing carryover apply-decisions behavior is unchanged: all pre-existing cmd/evolve carryover tests still pass"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestCarryover' ./cmd/evolve/..."
  - criterion: "cmd_carryover.go is gofmt clean and go vet ./cmd/evolve/... is clean after the extraction"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1753_007_TouchedFilesAreGofmtClean|TestC1753_008_TouchedPackageVetsClean' ./acs/cycle1753/..."
  - criterion: "The explanation document's Verification section reports the cycle predicate count go/acs/cycle1753 declares, and its evolve acs suite line carries the (cycle= regression= red-team=) breakdown with a cycle scope equal to that count, figures that add up (green+red+skip = total = cycle+regression+red-team) and red=0 under verdict=PASS"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1753_010_ExplanationDocVerificationCountsMatchDeclaredPredicates ./acs/cycle1753/..."
---

# Eval: shrink-carryover-apply

> Pins the size-ratchet extraction of `applyCarryoverDecisions`
> (`go/cmd/evolve/cmd_carryover.go`), 57 lines at cycle-1753 baseline, down to
> the `sizeratchet` ceiling of 50 by extracting its unlocked pre-read fast
> path into a standalone `carryoverNoOpFastPath` helper. Behavior must be
> preserved exactly: the fast path still short-circuits to a no-op result
> when no removal id is present in `carryoverTodos`, and still falls through
> to the locked `statemap.UpdateStateMap` read-modify-write otherwise (the
> original comment's own invariant — "an unlocked pre-read only skips a
> no-op write; UpdateStateMap re-reads under the lock, so it is never a
> correctness gate"). Source: cycle-1753 scout/triage carve-out of
> `go/internal/sizeratchet/offenders.json` lane work, task
> `shrink-carryover-apply`.
>
> Source incident: this is a mechanical, low-risk refactor task class; the
> score-cap pins both the structural outcome (line count, offenders.json
> entry removed) and the behavioral divergence the extraction must keep
> intact (hit vs. miss on the fast path), since a naive extraction can easily
> invert the boolean and silently turn every apply into an unconditional
> locked update or an unconditional no-op.
>
> Cycle-1753 audit round 2 (H1) failed the build on a narrative defect: the
> explanation document's suite line kept the round-1 totals after a predicate
> was added, contradicting its own predicate count; the verification-counts
> entry ties both figures to the predicates go/acs/cycle1753 declares.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | applyCarryoverDecisions <=50 lines | 8/10 | `go test -run TestC1753_001...` |
| wiring | applyCarryoverDecisions calls carryoverNoOpFastPath | 8/10 | `go test -run TestC1753_003...` |
| offenders-cleanup | offenders.json entry removed | 7/10 | `go test -run TestC1753_005...` |
| helper-tested | carryoverNoOpFastPath hit + fall-through covered | 7/10 | `go test -run 'TestCarryoverNoOpFastPath_'` |
| behavior-preserved | pre-existing carryover tests still pass | 8/10 | `go test -run 'TestCarryover'` |
| format-vet-clean | gofmt + go vet clean on cmd/evolve | 6/10 | `go test -run 'TestC1753_007...\|TestC1753_008...'` |
| verification-counts | doc Verification predicate and suite counts match the declared predicates | 6/10 | `go test -run TestC1753_010...` |

---
score_cap:
  - criterion: "The scoped set is the intersection of the reverse closures of the lane change and the new main commits, a global input selects the whole module, and a tree-reading test joins when its input changed"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestScopedSet_IsTheIntersectionOfBothReverseClosures|TestScopedSet_AGlobalInputSelectsAll|TestScopedSet_ATreeReadingTestJoinsWhenItsInputChanged)$' ./internal/scopedset"
  - criterion: "A red outside the scoped set is recorded as an escape, and the record is appended as one line under the plane"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestShadow_AFailureOutsideTheSetIsAnEscape|TestShadow_AppendWritesOneLinePerRecordUnderThePlane|TestShadow_AppendRefusesAnEmptyPlane)$' ./internal/scopedset"
  - criterion: "The composition gate runner records the shadow for a caller that names its site and still runs the full re-check, and a record that cannot be written changes no gate"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestComposedGatesTo_(ShadowRecords|AnAuditedBase|WithoutARecheckSite|AShadowRecordThatCannotBeWritten)' ./cmd/evolve"
  - criterion: "Both production re-check callers, the identity carry and the scoped merge, hand the gate runner the audited base and the project root"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRecheckSite_' ./internal/core"
  - criterion: "The new package is enrolled in the apicover gate and names every exported symbol"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2450_008_' ./acs/cycle2450"
---

# Eval: Scoped re-check at shadow with an escape record

> Pins component S8 of the plan docs/plans/gate-ship-retry-waste-2026-11.md (decision D8), built in cycle 2450. After main moves, a lane re-runs the composed gates in full: 8.25 hours in wave 174. S8 computes the packages that both the lane change and the new main commits can affect, still runs the full re-check, and records each failure outside that set as an escape. Stage 2 (enforce) waits for 2 waves with 0 escapes. The record file is `.evolve/scoped-recheck-shadow.ndjson` under the project root.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| set-algebra | intersection, global input, tree-reading join | 6/10 | `go test -run TestScopedSet_ ./internal/scopedset` |
| escape-record | escape judgement and one-line append | 7/10 | `go test -run TestShadow_ ./internal/scopedset` |
| runner-wiring | the real gate runner records the shadow and keeps the full re-check | 8/10 | `go test -run TestComposedGatesTo_ ./cmd/evolve` |
| caller-proof | both production callers name their site | 8/10 | `go test -run TestRecheckSite_ ./internal/core` |
| apicover | enrollment and named exports | 6/10 | `go test -tags acs -run TestC2450_008_ ./acs/cycle2450` |

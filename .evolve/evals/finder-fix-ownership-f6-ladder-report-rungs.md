---
score_cap:
  - criterion: "evolve ladder report names the defects each rung fixed, in JSON and in a text table"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2419_005|TestC2419_006' ./acs/cycle2419"
  - criterion: "retryladder.BuildReport counts a defect only for a passed non-R0 try"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2419_001|TestC2419_002|TestC2419_003' ./acs/cycle2419"
  - criterion: "a kept raised rung carries the defect it was raised for into its try record"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestLadderForDispatch_ ./internal/core"
---

# Eval: Finder fix ownership F6, the rung records in evolve ladder report

> Pins decision D7 of docs/plans/finder-fix-ownership-2026-10.md: each rung records which defect it fixed, and evolve ladder report counts them, so a rung that fixes nothing over 30 cases can be removed. Cycle 2419.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| report-cli | JSON and text name the defects per rung | 5/10 | `go test -tags acs -run 'TestC2419_005\|TestC2419_006' ./acs/cycle2419` |
| report-core | only a passed non-R0 try owns a defect | 6/10 | `go test -tags acs -run 'TestC2419_001\|TestC2419_002\|TestC2419_003' ./acs/cycle2419` |
| record-wiring | kept rung carries its fingerprint | 6/10 | `go test -run TestLadderForDispatch_ ./internal/core` |

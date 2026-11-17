---
score_cap:
  - criterion: "sync-main exits 0 on a tree whose only change is the loop's quarantine move, and main records the move before the merge"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2449_001' ./acs/cycle2449"
  - criterion: "Any other deletion or edit of a tracked inbox item still refuses with exit 3 and moves nothing"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2449_002' ./acs/cycle2449"
  - criterion: "The sync-main section of the boundary docs records the quarantine-move rule"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2449_003' ./acs/cycle2449"
---

# Eval: sync-main carries the loop's quarantine move

> Pins the rule that evolve sync-main records the loop's own quarantine move of a tracked inbox item and then merges, and that every other deletion or edit of a tracked inbox item still refuses. The code shipped in cycle 2169 (`eval sync-main-accepts-a-quarantine-move`). Cycle 2449 adds the end-to-end proof through the real binary and the docs rule that the operator reads at the boundary.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| move-recorded | exit 0, HEAD tracks only the quarantine copy, tree clean, origin merged | 4/10 | `go test -tags acs -run 'TestC2449_001' ./acs/cycle2449` |
| other-dirt-refused | four dirty shapes exit 3 and leave HEAD unchanged | 4/10 | `go test -tags acs -run 'TestC2449_002' ./acs/cycle2449` |
| docs-rule | a sync-main line names the quarantine move and its directory | 6/10 | `go test -tags acs -run 'TestC2449_003' ./acs/cycle2449` |

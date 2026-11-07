---
score_cap:
  - criterion: "a failed item records the rung of its try on the failure record"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2212_00[1-4]' ./acs/cycle2212"
  - criterion: "quarantine release clears the rung with the rest of the failure record"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2212_006' ./acs/cycle2212"
  - criterion: "the recorded rung reads back through the lifecycle reader and is a loop-owned stamp"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2212_00[78]' ./acs/cycle2212"
  - criterion: "every changed function of the writer and reader path has 100 percent coverage"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2212_(009|010|011)' ./acs/cycle2212"
---

# Eval: Retry ladder gap 1, the item failure record carries its rung

> Pins the plan gap of docs/plans/retry-ladder-2026-10.md section 5: the failure record of an item had no rung, so the D10 "R1 or higher" check could not run. The lane part stamps, clears, validates and reads the rung. The D10 consumer and the inboxmover facade are protected paths and stay console work (cycle 2146 AC 2). Source incident: cycles 2146, 2165, 2174 and 2192 did not ship this item.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stamp-written | a failed item records its rung | 4/10 | `TestC2212_001` to `TestC2212_004` |
| clean-release | quarantine release clears the rung | 6/10 | `TestC2212_006` |
| readable-owned | reader and loop-stamp role | 6/10 | `TestC2212_007`, `TestC2212_008` |
| coverage | 100% on changed functions | 7/10 | `TestC2212_009` to `TestC2212_011` |

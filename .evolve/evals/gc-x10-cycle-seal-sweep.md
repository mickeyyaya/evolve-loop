---
score_cap:
  - criterion: "A sealed cycle with a dirty lane and an orphan process is cleaned at once, after a saved and verified bundle"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2271_001' ./acs/cycle2271"
  - criterion: "A cycle that seals into a pause, a continuation or a live run keeps its lane, branch and processes"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2271_002' ./acs/cycle2271"
  - criterion: "A live lane of another cycle is never touched, and a failed bundle keeps the lane and the branch"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2271_003' ./acs/cycle2271"
  - criterion: "Both loop callers of a cycle end (sequential and resume) reach the sweep, and gc.mode governs it"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2271_004' ./acs/cycle2271"
  - criterion: "Each function of the new gc file and of the new loop file is covered 100%"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2271_005|TestC2271_006' ./acs/cycle2271"
  - criterion: "The fresh lease that the finished run leaves in the loop process does not hold the sweep, and a lease of another live process or a pause still does"
    max_if_missing: 2
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2271_008|TestC2271_009' ./acs/cycle2271"
---

# Eval: gc X10 cycle-seal sweep

> Pins component X10 of docs/plans/gc-closes-the-legacy-gap-2026-11.md (decisions D12 and D13). When cycle N seals, gc runs for cycle N only: it stops the orphan processes in the lane, saves the lane and verifies the bundle (D1, D2), then releases the lane and the cycle branch. It does not wait for salvage_after_hours, MinAgeMinutes or KeepRecent. A pause, a continuation binding or a live run holds the sweep. Source: cycle 2271, from the 2026-11-09 disk debt that grew until a manual run.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| sealed-cleaned | sealed dirty lane, orphan, bundle | 3/10 | `TestC2271_001` |
| holds | pause, continuation, live run keep the lane | 3/10 | `TestC2271_002` |
| others-untouched | live lane of another cycle, failed bundle | 4/10 | `TestC2271_003` |
| wired | sequential and resume callers reach the sweep | 3/10 | `TestC2271_004` |
| covered | 100% coverage of the new files | 6/10 | `TestC2271_005`, `TestC2271_006` |
| live-lease-at-end | the lease of the finished run, owned by the loop, does not hold the sweep (audit H1) | 2/10 | `TestC2271_008`, `TestC2271_009` |

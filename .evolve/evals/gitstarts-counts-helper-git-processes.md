---
score_cap:
  - criterion: "Each gittest helper that runs git is charged its measured process count by gitstarts.Count"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2638_001' ./acs/cycle2638"
  - criterion: "Every cost gitstarts charges comes from gittest.StartCosts and gitstarts holds no helper-name switch"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2638_002' ./acs/cycle2638"
  - criterion: "The guard names an exported gittest helper that starts git without a cost entry and the real gittest has none"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2638_003' ./acs/cycle2638"
  - criterion: "The recorded core ceiling is not raised and the core tests fit it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2638_004' ./acs/cycle2638"
---

# Eval: gitstarts charges gittest helper git processes

> Pins the repair of the git-starts ceiling. Before it, gitstarts.callCost held a fixed switch that charged 0 for gittest helpers it did not list, such as Repo.CloneBare and BranchExists. A helper that wraps git could then hide real starts under a recorded ceiling. The cost of each helper now lives in one table that gittest exports, and a guard fails when an exported helper that starts git has no entry. Source incident: cycle 2638, the CI fix of PR 1740 (core git starts 746 against a recorded 730).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| measured-charge | each helper charged its real git process count | 4/10 | `go test -tags acs -run TestC2638_001 ./acs/cycle2638` |
| table-is-ssot | every charged cost comes from gittest.StartCosts | 5/10 | `go test -tags acs -run TestC2638_002 ./acs/cycle2638` |
| guard | uncosted git-starting helper is named | 6/10 | `go test -tags acs -run TestC2638_003 ./acs/cycle2638` |
| ceiling | recorded ceiling not raised | 6/10 | `go test -tags acs -run TestC2638_004 ./acs/cycle2638` |

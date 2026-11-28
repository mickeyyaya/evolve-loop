---
score_cap:
  - criterion: "One delivered inbox item shows a delivered count of 1 in the queue view"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2667_001' ./acs/cycle2667"
  - criterion: "Every state of inboxmover.LifecycleStates has its own queue count"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2667_004' ./acs/cycle2667"
  - criterion: "The changed functions have 100% coverage"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2667_006' ./acs/cycle2667"
---

# Eval: The dashboard queue counts delivered items

> Pins the queue view of `internal/dashboard` to the state list of `internal/inboxmover`. A delivered item was invisible in the queue view because the dashboard kept its own hardcoded list of state directories (PR #1841 finding M-A). Source: inbox item dashboard-queue-counts-delivered-items, cycle 2667.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| delivered-count | one delivered item gives a count of 1 | 6/10 | `go test -tags acs -run TestC2667_001 ./acs/cycle2667` |
| state-list-parity | each listed state has a queue count | 7/10 | `go test -tags acs -run TestC2667_004 ./acs/cycle2667` |
| coverage | changed functions at 100% | 6/10 | `go test -tags acs -run TestC2667_006 ./acs/cycle2667` |

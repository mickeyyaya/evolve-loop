---
score_cap:
  - criterion: "A consumer never composes before its producer in any arrival order"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2042_001' ./acs/cycle2042"
  - criterion: "Two deliverables with no edge keep their order by rank, and rank beats every later key"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2042_002' ./acs/cycle2042"
  - criterion: "Each later key of the D3 score breaks the tie of the keys before it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2042_003' ./acs/cycle2042"
  - criterion: "The same input always gives the same order across every arrival order"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2042_004' ./acs/cycle2042"
  - criterion: "Empty, single, unknown-dep and cyclic batches are ordered cleanly"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2042_005' ./acs/cycle2042"
  - criterion: "The order leaves its input untouched and shares no storage with it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2042_006' ./acs/cycle2042"
  - criterion: "internal/integration is enrolled in .cover-strict and .apicover-enforce at 100% coverage with OrderDeliverables covered"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2042_007|TestC2042_008' ./acs/cycle2042"
---

# Eval: Wave integration I1 order score

> Pins the contract of `integration.OrderDeliverables`, the pure order score of decision D3 (ADR-0147). The score puts a producer before its consumer, then orders by inbox rank, the count of items unblocked, low risk, low verification cost, time of delivery and the identifier. Source incident: cycle 2042, component I1 of docs/plans/wave-integration-2026-10.md.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| deps-first | consumer after producer for every arrival order | 8/10 | `TestC2042_001` |
| rank-no-edge | no edge keeps rank order | 7/10 | `TestC2042_002` |
| key-sequence | later keys break ties in the D3 sequence | 6/10 | `TestC2042_003` |
| determinism | all arrival orders give one order | 8/10 | `TestC2042_004` |
| edge-cases | empty, single, unknown dep, cycle | 6/10 | `TestC2042_005` |
| purity | input unchanged, no shared storage | 6/10 | `TestC2042_006` |
| enrolment | strict cover and apicover at 100% | 6/10 | `TestC2042_007`, `TestC2042_008` |

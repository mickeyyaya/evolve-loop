---
score_cap:
  - criterion: "One failed cycle applied twice charges the item once"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2199_001|TestC2199_002|TestC2199_004' ./acs/cycle2199"
  - criterion: "Distinct failed cycles still charge one block each, and a host fault charges the task nothing"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2199_003|TestC2199_005' ./acs/cycle2199"
  - criterion: "ADR-0160 rule 10 states the count of ADR-0126 decision 16, and decision 16 keeps its cap"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2199_006|TestC2199_007' ./acs/cycle2199"
---

# Eval: ADR-0160 rule 10 and ADR-0126 decision 16 are one count

> Pins the operator ruling of ADR-0164 ruling 8: a charge is one block, and one event adds at most one block to a subject. A second `cycleoutcome.ApplyFailure` for the same cycle re-claims the released item through `ClaimLaneScope` and bumps `failure_count` again (cycle 2199 probe: count 1, then 2). The record must say the same as the code.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one-event-one-block | A repeat closeout of one cycle adds no block | 8/10 | `go test -tags acs -run TestC2199_001` |
| distinct-events-count | Two cycles charge two blocks; a host fault charges none | 7/10 | `go test -tags acs -run TestC2199_003` |
| record-agrees | ADR-0160 rule 10 names one block and decision 16 | 6/10 | `go test -tags acs -run TestC2199_006` |

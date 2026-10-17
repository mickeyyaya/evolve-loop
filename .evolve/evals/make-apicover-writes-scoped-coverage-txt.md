---
score_cap:
  - criterion: "make apicover writes its func report to the scoped coverage.apicover.func.txt, not the shared coverage.func.txt"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1939_001 ./acs/cycle1939"
  - criterion: "make apicover-check reads and writes only the scoped func report"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1939_002 ./acs/cycle1939"
  - criterion: "make cover keeps the shared coverage.func.txt"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1939_004 ./acs/cycle1939"
---

# Eval: make apicover writes scoped coverage files

> Pins that the apicover targets stop overwriting the coverage.func.txt that test-integration and cover produce. Source: cycle 1939 triage top_n.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| apicover-scoped | apicover func report scoped | 7/10 | `go test -tags acs -run TestC1939_001 ./acs/cycle1939` |
| apicover-check-scoped | apicover-check func report scoped | 7/10 | `go test -tags acs -run TestC1939_002 ./acs/cycle1939` |
| cover-preserved | cover keeps shared file | 5/10 | `go test -tags acs -run TestC1939_004 ./acs/cycle1939` |

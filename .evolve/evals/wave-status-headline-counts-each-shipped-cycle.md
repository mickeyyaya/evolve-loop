---
score_cap:
  - criterion: "The wave status headline, its cost line and the next wave goal give the same shipped count when a cycle ships late"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2342_001 ./acs/cycle2342"
  - criterion: "A shipped flag in run.json with no landing record counts in none of the three counts"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2342_002 ./acs/cycle2342"
---

# Eval: The wave status headline counts each shipped cycle

> Pins one shipped count for the wave status headline, the cost line and the next wave goal. Source incident: wave 166 printed "11 of 16 cycles shipped" in the headline and "12 of 16 shipped" in the cost line, because cycle 2333 shipped late. The goal of wave 167 copied the wrong count (cycle 2342).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| three-counts-agree | Headline, cost line and goal agree on a late ship | 4/10 | `go test -tags acs -run TestC2342_001 ./acs/cycle2342` |
| forged-flag | A planted shipped flag counts in none of the three | 6/10 | `go test -tags acs -run TestC2342_002 ./acs/cycle2342` |

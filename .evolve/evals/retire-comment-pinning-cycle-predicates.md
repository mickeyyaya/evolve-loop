---
score_cap:
  - criterion: "The four comment-pinning cycle packages are out of go/acs and archived with sources"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC1891_001|TestC1891_002)' ./acs/cycle1891"
  - criterion: "The archive README names each retired package with a reason"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1891_003' ./acs/cycle1891"
  - criterion: "cycle50 StrictPass predicate tolerates gofmt alignment and cycle1702 stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC1891_004|TestC1891_005|TestC1891_006|TestC1891_007)' ./acs/cycle1891"
---

# Eval: Retire comment-pinning cycle predicates

> Pins the retirement of the cycle 1741, 1742, 1759 and 1770 predicate packages
> that assert comment text or offenders.json diffs, and the whitespace-tolerant
> rewrite of TestC50B_006 in cycle50. Source incident: cycle 1891 scout report;
> inbox item retire-comment-pinning-cycle-predicates.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| archive | Four packages archived | 6/10 | `go test -run TestC1891_001,002` |
| readme | README lists reasons | 5/10 | `go test -run TestC1891_003` |
| cycle50 | Regex anchor, dependents green | 7/10 | `go test -run TestC1891_004..007` |

---
score_cap:
  - criterion: "the gc check report lists each pipeline temp entry with its shape: remove, keep_fresh, spare_held or spare_unknown, a temp.* rule and holder evidence"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2607_00[1234]_' ./acs/cycle2607"
  - criterion: "an empty temp dir gives an empty list, an unreadable one fails the check, an unset ttl plans no removal, and the check removes nothing"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2607_00[5678]_' ./acs/cycle2607"
  - criterion: "the temp reap keeps its rule and the existing tempdir tests pass unchanged"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2607_(009|010)_' ./acs/cycle2607"
---

# Eval: The gc check registry covers the pipeline temp reap

> Pins the temp check of `evolve gc check`: each pipeline temp entry appears with its shape, and the reap rule stays the one that `ReapPipelineTemp` applies. Source: inbox item recycle-x1d-check-registry-temp (plan PR #1585 D10, X1d), cycle 2607.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| report-shape | each entry listed with action, rule and evidence | 8/10 | `go test -tags acs -run 'TestC2607_00[1234]_' ./acs/cycle2607` |
| edge-and-negative | empty, unreadable, ttl unset, mutation-free | 7/10 | `go test -tags acs -run 'TestC2607_00[5678]_' ./acs/cycle2607` |
| preservation | reap unchanged, existing tests unchanged | 8/10 | `go test -tags acs -run 'TestC2607_(009|010)_' ./acs/cycle2607` |

---
score_cap:
  - criterion: "a fake backend with two false positives and one false negative scores exact precision and recall"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2294_001|TestC2294_002' ./acs/cycle2294"
  - criterion: "a truth that misses a real call fails the grep check and the run"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2294_003|TestC2294_004' ./acs/cycle2294"
  - criterion: "the table has precision, recall, p50, p95, bytes, failures and the load average"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2294_005|TestC2294_006|TestC2294_007' ./acs/cycle2294"
  - criterion: "evolve code bench is reachable from the command line and measures gotypes on the real index"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2294_009|TestC2294_010' ./acs/cycle2294"
---

# Eval: evolve code bench (code index G3)

> Pins the bench verb of plan code-index-backends-2026-10, component G3 (decisions D7 and D8). The bench scores each backend against the type-checked truth, and a grep check on a sample guards the truth itself. Source incident: cycle 2281 built the verb and failed on a full-suite abort in retro, so the work continues in cycle 2294.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exact-metrics | 2 false positives and 1 false negative give 0.6 and 0.75 | 6/10 | `TestC2294_001`, `TestC2294_002` |
| grep-check | a truth with a missing real call fails the run | 6/10 | `TestC2294_003`, `TestC2294_004` |
| table | all columns and the load average are printed | 7/10 | `TestC2294_005` to `TestC2294_007` |
| reachability | the real verb runs and records the gotypes values | 7/10 | `TestC2294_009`, `TestC2294_010` |

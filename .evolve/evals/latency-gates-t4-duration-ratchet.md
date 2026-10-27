---
score_cap:
  - criterion: "go/.test-latency.json is tracked and records a valid class and a positive p90 for every package that has tests"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2064_001 ./acs/cycle2064"
  - criterion: "testlatency -ratchet fails only above min(2 x class ceiling, -test-timeout) for each of the small, medium and large classes"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2064_00[234]' ./acs/cycle2064"
  - criterion: "a package above its recorded p90 by 50% and by more than 30 seconds is listed in the job-summary warning section, and no other package is"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2064_00[67]' ./acs/cycle2064"
  - criterion: "testlatency tighten lowers a recorded p90 and never raises one"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2064_(008|009|010)' ./acs/cycle2064"
  - criterion: "both go.yml jobs run the ratchet step against the committed file without swallowing its exit status"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2064_011 ./acs/cycle2064"
---

# Eval: duration ratchet for test packages (latency gates T4)

> Pins the T4 component of docs/plans/latency-gates-2026-10.md section 4.2 (cycle 2064): the committed
> go/.test-latency.json, the class-ceiling gate with a TEST_TIMEOUT cap, the p90 warning, and the
> tighten subcommand. Every predicate drives the real built binary of ./cmd/testlatency.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ratchet-file | class and p90 recorded for each tested package | 6/10 | `TestC2064_001` |
| class-ceiling | fail only above min(2 x ceiling, timeout) | 6/10 | `TestC2064_002..004` |
| p90-warning | warn at >50% and >30s over p90 | 7/10 | `TestC2064_006..007` |
| tighten | lower only, never raise | 7/10 | `TestC2064_008..010` |
| ci-wiring | both jobs gate on the committed file | 6/10 | `TestC2064_011` |

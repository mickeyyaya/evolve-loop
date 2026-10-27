---
score_cap:
  - criterion: "evolve ci telemetry prints the L5 to L1 critical path of a fixture of jobs, steps and test JSON"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2066_00[123]_' ./acs/cycle2066"
  - criterion: "the report joins the rusage and sampler records of the red package and names its limiting resource"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2066_00[4567]_' ./acs/cycle2066"
  - criterion: "the report gives the G1 macOS job time and the G1 ubuntu test step time"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2066_00[89]_' ./acs/cycle2066"
  - criterion: "bad input fails with a named error and prints no report"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2066_01[01]_' ./acs/cycle2066"
  - criterion: "every function of internal/citelemetry is covered at 100% and the package is enrolled in both coverage gates"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2066_01[23]_' ./acs/cycle2066"
---

# Eval: evolve ci telemetry, the critical-path report (C1)

> Pins the contract of `evolve ci telemetry` (plan ci-wall-time-2026-10, component C1, decisions D3 and D12). The report joins the GitHub jobs and steps, the test JSON stream, the rusage records and the sampler records of one run, and names the critical path from L5 to L1 with the limiting resource of each package. Source incident: the C0 and C0b artifacts exist, but no verb joined them (cycle 2066 scout).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| path | L5 to L1 path from the fixture | 4/10 | `TestC2066_001` to `003` |
| join | rusage and sampler join, limit rules | 5/10 | `TestC2066_004` to `007` |
| g1 | G1 macOS job and ubuntu test step | 6/10 | `TestC2066_008`, `009` |
| loud failure | bad input exits non-zero with a named error | 7/10 | `TestC2066_010`, `011` |
| coverage | 100% per function, both gates | 6/10 | `TestC2066_012`, `013` |

---
score_cap:
  - criterion: "The TestCoreCoverageBatch_ tests alone take worktreeBaseTreeSHA, recordBuildBinding, recordGenericBinding, packageNewThisCycle, writeBuildSelfCheckArtifact and contentionBackoff to 100.0% statement coverage"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1938_007_' ./acs/cycle1938 | grep -q -- '--- PASS: TestC1938_007_'"
  - criterion: "The unmet remainder (the rest of internal/core to 100.0% and the go/.cover-strict registration) is a tracked, claimable, weighted inbox-root item linked by connects_to/deps"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1938_008_' ./acs/cycle1938 | grep -q -- '--- PASS: TestC1938_008_'"
---

# Eval: internal/core coverage batch — phase bindings and ship-recovery backoff

> Pins one batch of `core-package-coverage-to-100`. On 2026-10-18 the package measured 96.8%
> with `-tags integration` and 110 functions below 100.0%. This batch takes the six functions of
> `phase_bindings*.go` and `ship_recovery_budget.go` that are below 100.0% to full coverage
> with behavior tests (error branches are the negative cases; the backoff cap is the edge case).
> The batch does not register `./internal/core` in `go/.cover-strict`: the package is not at
> 100.0% yet. Source: inbox item from console lane cl-lq-q1 (ADR-0128 Q1).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| batch-coverage | six named functions at 100.0% | 8/10 | `TestC1938_007_` |
| remainder | queued tracked remainder item | 8/10 | `TestC1938_008_` |

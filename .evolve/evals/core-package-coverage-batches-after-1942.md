---
score_cap:
  - criterion: "The TestCoreCoverageBatch_ tests alone take materializeLaneScope, scoutReportGoalHash, buildBudgetScale and SeedDispositionSkeleton to 100.0% statement coverage"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1944_001_' ./acs/cycle1944 | grep -q -- '--- PASS: TestC1944_001_'"
  - criterion: "The unmet remainder (the rest of internal/core to 100.0% and the go/.cover-strict registration) is a new tracked, claimable, weighted inbox-root item linked to core-package-coverage-batches-after-1942"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1944_002_' ./acs/cycle1944 | grep -q -- '--- PASS: TestC1944_002_'"
---

# Eval: internal/core coverage batch — lane scope, build budget scale and disposition seed

> Pins one batch of `core-package-coverage-batches-after-1942`. On 2026-10-18 (cycle 1944 TDD) the
> package measured 97.6% with `-tags integration`. This batch takes `materializeLaneScope` and
> `scoutReportGoalHash` of `lanescope.go`, `buildBudgetScale` of `size_budget.go` and
> `SeedDispositionSkeleton` of `disposition_seed.go` to 100.0% with behavior tests. The negative
> cases are an unwritable workspace, a missing scout report, a missing workspace for the router
> digest, a missing or empty ancestor ledger and a failed seed write. The batch does not register
> `./internal/core` in `go/.cover-strict`, because the package is not at 100.0% yet. The protected
> `phase_bindings_selfcheck.go` stays untouched.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| batch-coverage | four named functions at 100.0% | 8/10 | `TestC1944_001_` |
| remainder | new queued tracked remainder item | 8/10 | `TestC1944_002_` |

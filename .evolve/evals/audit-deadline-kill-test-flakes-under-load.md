---
score_cap:
  - criterion: "TestDecideTier_DeadlineHitRequiresSynchronizedCtx exists in package audit and passes 20/20 consecutive runs"
    max_if_missing: 6
    evidence: "cd go && go test -count=20 -run '^TestDecideTier_DeadlineHitRequiresSynchronizedCtx$' -v ./internal/phases/audit/ 2>&1 | grep -c '^--- PASS: TestDecideTier_DeadlineHitRequiresSynchronizedCtx ' | grep -qx 20"
  - criterion: "The regression test fails deterministically (5/5) when killedAtDeadline stops waiting on ctx.Done()"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1727_002_RegressionTestDeterministicallyCatchesDesynchronizedHelper$' ./acs/cycle1727/"
  - criterion: "The regression test reaches the production deadline read (ciparitygate.runAttempt) through the real integration-tier seam"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1727_003_RegressionTestDrivesProductionDeadlineRead$' ./acs/cycle1727/"
  - criterion: "Every DeadlineKill test in package audit passes 50/50 consecutive runs"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1727_004_DeadlineKillTestsGreenOver50Runs$' ./acs/cycle1727/"
---

# Eval: Regression-lock the deadline-kill determinism fix

> `TestAuditOrchestration_IntegrationTier_DeadlineKill_MarkerFreeDegradesToWarn`
> flaked twice under whole-module load on 2026-09-13: a scripted runner that
> returned instantly could be observed before `context.WithTimeout(…, 1ns)`
> had set `ctx.Err()`, so `runAttempt` recorded `deadlineHit=false` and the
> marker-free budget WARN became a red-retake FAIL. `killedAtDeadline`
> (`go/internal/phases/audit/ciparity_unit_test.go`) fixed it by making the
> fake return only after `<-ctx.Done()`, but nothing asserted that the wait
> is load-bearing. This eval pins a regression test that fails
> deterministically when the wait is dropped (no timer race needed to
> notice) and that drives the real integration-tier seam, so a broken
> production deadline read also fails it. Source incident: inbox item
> `audit-deadline-kill-test-flakes-under-load`, worked in cycle 1727 (TDD
> mutation probe: desynchronized helper reproduced the race at run 16/40).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| regression-present | New test passes 20/20 | 6/10 | `go test -count=20 -run '^TestDecideTier_DeadlineHitRequiresSynchronizedCtx$'` |
| load-bearing-sync | Fails 5/5 with the `<-ctx.Done()` wait removed (overlay mutant) | 8/10 | `TestC1727_002_…` |
| production-reach | Fails when `runAttempt` stops recognising `DeadlineExceeded` (overlay mutant) | 7/10 | `TestC1727_003_…` |
| inbox-acceptance | DeadlineKill tests 50/50 | 6/10 | `TestC1727_004_…` |

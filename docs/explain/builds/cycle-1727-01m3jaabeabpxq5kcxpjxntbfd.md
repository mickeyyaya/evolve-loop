# Build Explanation — Cycle 1727

## Build Binding
- Cycle: 1727
- Base SHA: a4d083e626d54bc431c77909a0197cfa594f5e31

## Summary
The deadline-kill determinism fix now has a regression test, `TestDecideTier_DeadlineHitRequiresSynchronizedCtx` in package audit. The fix made the scripted runner `killedAtDeadline` return only after `<-ctx.Done()`. Before this change, nothing asserted that the wait was load-bearing. Deleting it would only have brought back the intermittent whole-module-load flake in the DeadlineKill tests, not a deterministic failure. The new test fails every run when the wait is removed. It also fails every run when the production deadline read in `ciparitygate.runAttempt` stops recognising `context.DeadlineExceeded`.

## Rationale
`runAttempt` records `deadlineHit` from `ctx.Err()` at the moment the runner returns. A 1 ns `context.WithTimeout` can arm a timer instead of expiring synchronously. If the fake returns instantly, it can be observed before `ctx.Err()` is set, and then a marker-free budget WARN becomes a red-retake FAIL. A test that only reruns the DeadlineKill tests many times would catch a regression only probabilistically. So the first subtest uses a context wrapper that reports the first request for its `Done` channel. The test then `select`s between "the runner already ran" (the raced shape, which is fatal) and "the helper is waiting on Done". This detects the regression with no timers and no dependence on the scheduler. One alternative, the TDD prototype's 40-iteration production-seam loop, was rejected: the deterministic probe already gives a certain signal, and the loop only added runtime and a flake surface.

## Changed Areas
- `go/internal/phases/audit/integration_tier_orchestration_test.go` — adds `doneProbeCtx` (a `Done()`-reporting context wrapper), the shared `markerFreeKillScript`, and `TestDecideTier_DeadlineHitRequiresSynchronizedCtx`, which has three subtests: a deterministic probe of the helper's ordering, a synchronized kill reaching the budget WARN through `classifyThroughProductionIntegrationTierGate`, and a contrast arm showing that an un-killed runner with the same output is a red-retake FAIL. The file was chosen because the test sits beside the DeadlineKill tests it guards and reuses their fixtures.
- `go/acs/cycle1727/predicates_test.go` — the TDD phase's acceptance predicates, including the build-time overlay mutants for the helper and for `runAttempt`. They are committed unchanged with the test they verify.
- `.evolve/evals/audit-deadline-kill-test-flakes-under-load.md` — the TDD phase's eval for this task, committed unchanged with the cycle.

## Design Decisions
The test reuses the package's existing `killedAtDeadline` instead of copying it, so any change to the real helper is what the test judges. The helper keeps its `<-ctx.Done()` as a standalone statement. The contrast arm pins the budget WARN to the deadline, not to marker-free output alone, so the production-seam subtest cannot pass for the wrong reason. The existing DeadlineKill tests were left untouched to keep the diff minimal.

## Verification
- `go test -count=20 -run '^TestDecideTier_DeadlineHitRequiresSynchronizedCtx$' ./internal/phases/audit/` passes.
- All six `go/acs/cycle1727` predicates are green, including the 5/5 helper-mutant failure and the 3/3 `runAttempt`-mutant failure.
- `-race -count=10` over the new test and the DeadlineKill tests passes.
- The full `internal/phases/audit/...` suite passes.
- DeadlineKill at 50× and the new test at 20× passed in three rounds while a whole-module `go test ./...` ran concurrently.

## Compatibility
The change is test-only. `ciparity.go` and `ciparitygate/` have no diff against the base, and no exported API changes.

## Limitations
The test covers the scripted-runner contract and the production deadline read. It does not measure real subprocess kill timing, which stays outside unit-test scope.

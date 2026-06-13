---
score_cap:
  - criterion: "TestRunLoop_FailVerdictBreaks isolates from the ambient EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS var — it passes even when that var is set to 3 in the environment"
    max_if_missing: 7
    evidence: "cd go && EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 go test ./cmd/evolve/... -run TestRunLoop_FailVerdictBreaks -count=1 -short"
  - criterion: "the full cmd/evolve suite is green even with EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 set in the environment"
    max_if_missing: 6
    evidence: "cd go && EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 go test ./cmd/evolve/... -count=1 -short"
---

# Eval: cmd-loop-fail-breaker-isolation

> Pins the environment-hermeticity of `TestRunLoop_FailVerdictBreaks` in
> `go/cmd/evolve/cmd_loop_coverage_test.go`. The test asserts the loop trips its
> stop-on-first-fail circuit breaker (rc=2, stop_reason=fail) when audit returns
> FAIL. But `resolveMaxConsecutiveFails()` reads `EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS`,
> and the test did not isolate from it — so on an operator host where that var is
> set to 3, the loop continued past both FAIL cycles and exited via max_cycles
> (rc=3), turning the suite red. Source incident: cycle 318 scout health check
> (`go test ./... -short` → 1 FAIL in cmd/evolve, rc=3 want 2). The fix isolates
> the test from the ambient var (e.g. `t.Setenv("EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS",
> "1")`); both evidence commands INJECT the var at value 3 so they verify the
> isolation rather than relying on a clean environment.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| env-isolation | Targeted test passes under ambient var=3 | 7/10 | `EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 go test -run TestRunLoop_FailVerdictBreaks` |
| no-regression | Full cmd/evolve suite green under ambient var=3 | 6/10 | `EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS=3 go test ./cmd/evolve/...` |

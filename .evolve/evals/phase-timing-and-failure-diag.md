---
score_cap:
  - criterion: "phase-timing.json is written after a cycle with the 4-field per-phase schema in execution order"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestPhaseTimingJSONWritten$' -v ./internal/core/ 2>&1 | grep -q 'PASS: TestPhaseTimingJSONWritten'"
  - criterion: "<phase>-failure-diag.json is written before a non-recoverable abort returns, with phase/cycle/error_message/exit_code/attempt_count/verdict=FAIL/timestamp"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestFailureDiagWrittenOnAbort$' -v ./internal/core/ 2>&1 | grep -q 'PASS: TestFailureDiagWrittenOnAbort'"
  - criterion: "the whole core package still compiles and passes (no cycle-162 undefined-symbol leak)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/core/..."
---

# Eval: phase-timing.json + structured failure diagnostics

> Pins the two observability artifacts added to `core.RunCycle` in cycle 164:
> a deferred `phase-timing.json` (per-phase `{phase,duration_ms,verdict,cost_usd}`
> in execution order) and a `<phase>-failure-diag.json` written immediately before
> the orchestrator aborts a non-recoverable phase. These turn silent per-phase
> latency and silent aborts into inspectable artifacts.
>
> Source incident: cycle 162 (2026) claimed to modify `orchestrator.go` but the
> worktree only staged test files referencing undefined symbols (`phaseTimingEntry`,
> `failureDiagPayload`) — Audit FAIL. This eval enforces that the production code +
> a passing behavioral test both exist, not just a test stub. The grep-for-`PASS`
> evidence form is deliberate: a bare `go test -run` exits 0 on "no tests to run"
> (false green), so the eval requires the named test to actually emit a PASS line.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| timing-artifact | phase-timing.json schema + order | 6/10 | `go test -run TestPhaseTimingJSONWritten -v ... \| grep PASS` |
| failure-diag | diag file + 7-field schema on abort | 7/10 | `go test -run TestFailureDiagWrittenOnAbort -v ... \| grep PASS` |
| no-symbol-leak | core package compiles + passes | 5/10 | `go test ./internal/core/...` |

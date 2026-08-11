# Eval: short-gate-subprocess-tests

## Summary
`testing.Short()` early-exit guards added to the tests in `internal/phases/audit`,
`internal/rollback`, and `internal/releasepreflight` that shell out to real
`go test`, `git`, `gh`, or `bash` subprocesses. Full CI still runs them;
`go test -short` skips them for fast development iteration.

## Acceptance Criteria

### AC-1: audit subprocess tests skip under -short
```bash [code]
cd go && go test -count=1 -short -v ./internal/phases/audit/... 2>&1 | grep -E "SKIP|skip|--- SKIP"
```
Expected: at least the `TestNewDefault_WiresVerdictGenerator` and
`TestGenerateACSVerdict_EmptyWorktree_FallsBackToProjectRoot` tests appear as SKIP.

### AC-2: audit full run still has all tests pass
```bash [code]
cd go && go test -count=1 ./internal/phases/audit/... 2>&1; echo "exit:$?"
```
Expected: `ok ... exit:0` — no tests skipped (full suite).

### AC-3: rollback subprocess tests skip under -short
```bash [code]
cd go && go test -count=1 -short -v ./internal/rollback/... 2>&1 | grep -E "SKIP|skip|--- SKIP"
```
Expected: at least one `--- SKIP` line for a `defaultRevertAndShip` or
`defaultDeleteRemoteTag` test.

### AC-4: rollback full run still passes
```bash [code]
cd go && go test -count=1 ./internal/rollback/... 2>&1; echo "exit:$?"
```
Expected: `ok ... exit:0`.

### AC-5: audit -short completes in under 1.5s (down from ~3.7s)
```bash [code]
cd go && go clean -testcache && start=$(date +%s) && go test -count=1 -short ./internal/phases/audit/... 2>&1 | tail -1 && echo "elapsed:$(($(date +%s)-start))"
```
Expected: elapsed ≤ 2 seconds.

### AC-6: Negative — no testing.Short guard added to a pure-inmemory test
```bash [code]
cd go && grep -n "testing.Short" internal/phases/audit/audit_test.go | head -5
```
Expected: only lines inside tests that call `generateACSVerdict` or `writeGoPredFixture`
(the real-subprocess tests). Pure in-memory tests (TestExtractPrefersSentinel, etc.)
must not be gated.

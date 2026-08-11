# Eval: parallel-core-observer-tests

## Summary
`t.Parallel()` added to the majority of test functions in `internal/core` and
`internal/adapters/observer` that currently run sequentially, allowing concurrent
execution. Tests that write shared mutable state (TestMain seams, backoffSleep
save/restore) are left sequential.

## Acceptance Criteria

### AC-1: observer time.Sleep tests now call t.Parallel
```bash [code]
cd go && grep -l "time\.Sleep" internal/adapters/observer/*_test.go | xargs grep -L "t\.Parallel()"
```
Expected: empty output — every observer test file containing `time.Sleep` now also
contains `t.Parallel()`.

### AC-2: observer tests pass with -race
```bash [code]
cd go && go test -count=1 -race ./internal/adapters/observer/... 2>&1; echo "exit:$?"
```
Expected: `ok ... exit:0` with no DATA RACE output.

### AC-3: observer completes faster than 3s (down from ~6.6s)
```bash [code]
cd go && go clean -testcache && start=$(date +%s) && go test -count=1 ./internal/adapters/observer/... 2>&1 | tail -1 && echo "elapsed:$(($(date +%s)-start))"
```
Expected: elapsed ≤ 4 seconds.

### AC-4: core tests pass with -race
```bash [code]
cd go && go test -count=1 -race ./internal/core/... 2>&1; echo "exit:$?"
```
Expected: `ok ... exit:0` with no DATA RACE output.

### AC-5: core test files with no prior t.Parallel now have t.Parallel
```bash [code]
cd go && grep -rL "t\.Parallel()" internal/core/*_test.go 2>/dev/null | wc -l
```
Expected: fewer files without `t.Parallel()` than before (baseline was ~50 files
without any parallel call; after change ≤ 10 sequential-only files should remain —
only the backoff-unit tests and any test using shared mutable seams).

### AC-6: Negative — backoff unit tests must NOT call t.Parallel
```bash [code]
cd go && grep -n "t\.Parallel\|backoffSleep" internal/core/orchestrator_backoff_test.go 2>/dev/null | head -10; echo "exit:$?"
```
Expected: `backoffSleep` references present, zero `t.Parallel()` lines — the
save/restore tests around the shared `backoffSleep` seam must stay sequential.

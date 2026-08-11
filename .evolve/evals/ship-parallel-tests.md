# Eval: ship-parallel-tests

## Summary
`t.Parallel()` added to `internal/phases/ship` test functions so the 255 independent
git-repo tests run concurrently instead of sequentially.

## Acceptance Criteria

### AC-1: All ship tests call t.Parallel() (except t.Setenv user)
```bash [code]
cd go && grep -L "t\.Parallel()" internal/phases/ship/*_test.go | grep -v "testmain_test.go\|final_push_test.go"
```
Expected: empty output (every other test file contains at least one `t.Parallel()` call).

### AC-2: final_push_test.go test with t.Setenv must NOT call t.Parallel()
```bash [code]
cd go && grep -A5 "TestRun_BypassShipVerify_NilEnvMap_InitialisedInBypass" internal/phases/ship/final_push_test.go | grep "t\.Parallel()"
```
Expected: empty output (the env-mutating test must stay sequential).

### AC-3: All ship tests pass with -race
```bash [code]
cd go && go test -count=1 -race ./internal/phases/ship/... 2>&1; echo "exit:$?"
```
Expected: `ok` line with `exit:0`. No `DATA RACE` in output.

### AC-4: Ship package completes faster than 15s (down from ~26s)
```bash [code]
cd go && go clean -testcache && start=$(date +%s) && go test -count=1 ./internal/phases/ship/... 2>&1 | tail -1 && echo "elapsed:$(($(date +%s)-start))"
```
Expected: elapsed ≤ 15 seconds.

### AC-5: Negative — sequential test still does not call t.Parallel
```bash [code]
cd go && grep -c "t\.Parallel()" internal/phases/ship/testmain_test.go
```
Expected: `0` — TestMain file has no t.Parallel calls (it is not a test function).

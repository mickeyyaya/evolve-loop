# Eval: rollback-default-fns-coverage

## Task
Improve test coverage of `go/internal/rollback` from 86.8% to ≥92% by adding tests for the injected-seam functions `deleteRemoteTagWith` and `revertAndShipWith` using mock git executors.

## Acceptance Criteria

### AC1: Coverage floor [code]
```bash
cd go && go test -cover ./internal/rollback/... 2>&1 | grep -E "coverage: [0-9]" | awk -F'coverage: ' '{print $2}' | awk '{if ($1+0 >= 92.0) print "PASS"; else print "FAIL: coverage "$1}'
```
Expected: `PASS`

### AC2: No regression — all existing tests still pass [code]
```bash
cd go && go test ./internal/rollback/... 2>&1 | tail -3
```
Expected: output contains `ok` and no `FAIL`

### AC3: Negative case — deleteRemoteTagWith returns "failed" when push fails [code]
```bash
cd go && go test ./internal/rollback/... -run TestDeleteRemoteTagWith -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `--- PASS` present

### AC4: Edge case — revertAndShipWith returns "failed" when git revert fails [code]
```bash
cd go && go test ./internal/rollback/... -run TestRevertAndShipWith -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `--- PASS` present

### AC5: Edge case — deleteRemoteTagWith returns "not-present" when tag not on remote [code]
```bash
cd go && go test ./internal/rollback/... -run TestDeleteRemoteTagWithNotPresent -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `--- PASS` present

# Eval: skillcheck-run-coverage

## Task
Add tests for the `Run` function and uncovered edge paths in `go/internal/skillcheck`.

## Acceptance Criteria

### AC-1: `Run` check-mode on clean repo returns 0 [code]
```bash
cd go && go test -run TestRun_NoDriftCheckMode ./internal/skillcheck/... -v 2>&1 | grep -q "PASS"
```

### AC-2: `Run` write-mode on clean repo returns 0 [code]
```bash
cd go && go test -run TestRun_NoDriftWriteMode ./internal/skillcheck/... -v 2>&1 | grep -q "PASS"
```

### AC-3: `Run` check-mode with drift returns 2 and emits DRIFT to stderr [code]
```bash
cd go && go test -run TestRun_DriftCheckMode ./internal/skillcheck/... -v 2>&1 | grep -q "PASS"
```

### AC-4: `SpliceMarkedRegion` appends to EOF when no markers and no anchor [code]
```bash
cd go && go test -run TestSpliceMarkedRegion_AppendToEOF ./internal/skillcheck/... -v 2>&1 | grep -q "PASS"
```

### AC-5: `parallelSubtaskCount` returns 0 for invalid JSON [code]
```bash
cd go && go test -run TestParallelSubtaskCount_InvalidJSON ./internal/skillcheck/... -v 2>&1 | grep -q "PASS"
```

### AC-6: Coverage lifts from 69.2% to ≥78% [code]
```bash
cd go && go test -cover ./internal/skillcheck/... 2>&1 | grep -E "coverage: [0-9.]+" | awk -F'[: %]' '{if ($3+0 >= 78.0) print "OK"; else print "FAIL coverage too low: " $0}'
```

### AC-NEG: All existing tests still pass (no regression) [code]
```bash
cd go && go test ./internal/skillcheck/... 2>&1 | tail -1 | grep -q "^ok"
```

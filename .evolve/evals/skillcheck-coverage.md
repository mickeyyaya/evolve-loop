# Eval: skillcheck-coverage

## Task
Improve test coverage of `go/internal/skillcheck` from 82.7% to ≥90% by adding tests for uncovered branches in `nameMismatches`, `parallelSubtaskCount`, `inspect`, and `collectSkillFacts`.

## Acceptance Criteria

### AC1: Coverage floor [code]
```bash
cd go && go test -cover ./internal/skillcheck/... 2>&1 | grep -E "coverage: [0-9]" | awk -F'coverage: ' '{print $2}' | awk '{if ($1+0 >= 90.0) print "PASS"; else print "FAIL: coverage "$1}'
```
Expected: `PASS`

### AC2: No regression — all existing tests still pass [code]
```bash
cd go && go test ./internal/skillcheck/... 2>&1 | tail -3
```
Expected: output contains `ok` and no `FAIL`

### AC3: Negative case — nameMismatches detects name drift [code]
```bash
cd go && go test ./internal/skillcheck/... -run TestNameMismatches -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `--- PASS` present

### AC4: Edge case — parallelSubtaskCount handles missing key [code]
```bash
cd go && go test ./internal/skillcheck/... -run TestParallelSubtaskCount -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: `--- PASS` present

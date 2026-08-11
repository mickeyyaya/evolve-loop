# Eval: codequality-firstline-coverage

## Task
Add tests for the uncovered `firstLine` edge case and `UnformattedGoFiles` on empty dir in `go/internal/codequality`.

## Acceptance Criteria

### AC-1: `firstLine` with no newline returns the whole string [code]
```bash
cd go && go test -run TestFirstLine ./internal/codequality/... -v 2>&1 | grep -q "PASS"
```

### AC-2: `UnformattedGoFiles` on empty dir returns empty slice, no error [code]
```bash
cd go && go test -run TestUnformattedGoFiles_EmptyDir ./internal/codequality/... -v 2>&1 | grep -q "PASS"
```

### AC-3: Coverage lifts from 86.4% to ≥88% [code]
```bash
cd go && go test -cover ./internal/codequality/... 2>&1 | grep -E "coverage: [0-9.]+" | awk -F'[: %]' '{if ($3+0 >= 88.0) print "OK"; else print "FAIL coverage too low: " $0}'
```

### AC-NEG: Existing tests still pass [code]
```bash
cd go && go test ./internal/codequality/... 2>&1 | tail -1 | grep -q "^ok"
```

### AC-EDGE: `firstLine` with multi-line string returns only first line [code]
```bash
cd go && go test -run TestFirstLine ./internal/codequality/... -v 2>&1 | grep -q "PASS"
```

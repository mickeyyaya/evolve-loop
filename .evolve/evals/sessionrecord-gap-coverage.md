# Eval: sessionrecord gap coverage

## Acceptance Criteria

### AC-1: Append write-error path is tested via injected failing writer [code]
```bash
cd go && go test ./internal/sessionrecord/... -run TestAppend_WriteError -v -count=1 2>&1 | grep -E "PASS|FAIL"
# Expected: contains "PASS"
```

### AC-2: Append close-error path is tested [code]
```bash
cd go && go test ./internal/sessionrecord/... -run TestAppend_CloseError -v -count=1 2>&1 | grep -E "PASS|FAIL"
# Expected: contains "PASS"
```

### AC-3: Coverage climbs to >= 97% [code]
```bash
cd go && go test -count=1 -cover ./internal/sessionrecord/... 2>&1 | grep "coverage:"
# Expected: coverage: 9[7-9]\.[0-9]%
```

### AC-4: All sessionrecord tests pass [code]
```bash
cd go && go test -count=1 ./internal/sessionrecord/... 2>&1 | grep -E "^(ok|FAIL)"
# Expected: "ok" (not "FAIL")
```

### NEGATIVE: Append must return an error on write failure, not silently succeed [code]
```bash
cd go && go test ./internal/sessionrecord/... -run TestAppend_WriteError -v -count=1 2>&1 | grep "no error" | wc -l
# Expected: 0 (test does not see an unexpected nil error)
```

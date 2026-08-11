# Eval: codequality gap coverage

## Acceptance Criteria

### AC-1: firstLine handles strings with no newline [code]
```bash
cd go && go test ./internal/codequality/... -run TestFirstLine -v -count=1 2>&1 | grep -E "PASS|FAIL"
# Expected: contains "PASS" (test exists and passes)
```

### AC-2: UnformattedGoFiles returns error when gofmt binary is missing [code]
```bash
cd go && go test ./internal/codequality/... -run TestUnformattedGoFiles_BinaryMissing -v -count=1 2>&1 | grep -E "PASS|FAIL"
# Expected: contains "PASS"
```

### AC-3: Coverage climbs to >= 95% [code]
```bash
cd go && go test -count=1 -cover ./internal/codequality/... 2>&1 | grep "coverage:"
# Expected: coverage: 9[5-9]\.[0-9]% (or 100%)
```

### AC-4: All existing codequality tests still pass [code]
```bash
cd go && go test -count=1 ./internal/codequality/... 2>&1 | grep -E "^(ok|FAIL)"
# Expected: "ok" (not "FAIL")
```

### NEGATIVE: Missing-binary case must not succeed silently [code]
```bash
cd go && go test -count=1 -run TestUnformattedGoFiles_BinaryMissing ./internal/codequality/... 2>&1 | grep -v "PASS" | grep "FAIL" | wc -l
# Expected: 0 (test passes, no FAILs)
```

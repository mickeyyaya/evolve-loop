# Eval: soakreport gap coverage

## Acceptance Criteria

### AC-1: appendOnce dedup branch is tested [code]
```bash
cd go && go test ./internal/soakreport/... -run TestAppendOnce -v -count=1 2>&1 | grep -E "PASS|FAIL"
# Expected: contains "PASS"
```

### AC-2: Render emits notes when a workspace is unreadable [code]
```bash
cd go && go test ./internal/soakreport/... -run TestRender_WithNotes -v -count=1 2>&1 | grep -E "PASS|FAIL"
# Expected: contains "PASS"
```

### AC-3: collectInteractions file-read error path is covered [code]
```bash
cd go && go test ./internal/soakreport/... -run TestCollect_InteractionsFileUnreadable -v -count=1 2>&1 | grep -E "PASS|FAIL"
# Expected: contains "PASS"
```

### AC-4: Coverage climbs to >= 97% [code]
```bash
cd go && go test -count=1 -cover ./internal/soakreport/... 2>&1 | grep "coverage:"
# Expected: coverage: 9[7-9]\.[0-9]%
```

### AC-5: All soakreport tests pass [code]
```bash
cd go && go test -count=1 ./internal/soakreport/... 2>&1 | grep -E "^(ok|FAIL)"
# Expected: "ok" (not "FAIL")
```

### NEGATIVE: appendOnce must not add a duplicate note [code]
```bash
cd go && go test ./internal/soakreport/... -run TestAppendOnce_Deduplicates -v -count=1 2>&1 | grep "FAIL" | wc -l
# Expected: 0
```

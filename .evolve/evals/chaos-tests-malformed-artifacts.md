# Eval: Chaos Tests for Malformed Artifacts + Floor Boundary

## Acceptance Criteria

### AC-1: Binary content before header test exists in backfill [code]
```bash
grep -c "BinaryContent\|binary.*header\|NUL\|binary_content" internal/backfill/backfill_test.go
# Expected: >= 1 (new chaos test present)
```

### AC-2: Duplicate-header last-wins test exists in backfill [code]
```bash
grep -c "DuplicateHeader\|duplicate.*header\|LastWins\|two.*header\|MultipleHeader" internal/backfill/backfill_test.go
# Expected: >= 1 (new test present)
```

### AC-3: Floor boundary test (audit-present build-absent) exists [code]
```bash
grep -c "AuditPresentBuildAbsent\|audit.*build.*absent\|BuildAbsent.*Audit\|audit_without_build" internal/router/floor_test.go
# Expected: >= 1 (new negative boundary test present)
```

### AC-4: Backfill tests still pass at 100% [code]
```bash
cd go && go test -cover ./internal/backfill/ -count=1 -timeout 30s 2>&1 | grep -E "coverage:|FAIL"
# Expected: "coverage: 100.0%" and no FAIL
```

### AC-5: Router tests still pass [code]
```bash
cd go && go test ./internal/router/ -count=1 -timeout 60s 2>&1 | grep -E "^ok|FAIL"
# Expected: "ok" (not "FAIL")
```

### AC-6: Binary content chaos test verifies graceful false return [code]
```bash
cd go && go test ./internal/backfill/ -run "Binary\|NUL\|Chaos" -v -count=1 2>&1 | grep -E "PASS|FAIL|==="
# Expected: PASS on all matching tests, no FAIL
```

### AC-7: Floor boundary negative test verifies build is forced [code]
```bash
cd go && go test ./internal/router/ -run "AuditPresent\|BuildAbsent\|Boundary" -v -count=1 2>&1 | grep -E "PASS|FAIL|==="
# Expected: PASS on matching tests, no FAIL
```

### NEGATIVE: TryExtract with binary-only content returns (false, nil) [code]
```bash
# Verified by TestTryExtract_BinaryContentBeforeHeader test:
# NUL bytes before header → header still found → extracts content after header
# OR: pure binary (no header) → (false, nil) gracefully
cd go && go test ./internal/backfill/ -run "Binary" -v -count=1 2>&1
# Expected: PASS (no FAIL or panic)
```

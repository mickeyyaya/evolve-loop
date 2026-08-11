# Eval: ACS FilePresent Migration + Cycle106 Version-Pin Fix

## Acceptance Criteria

### AC-1: cycle43 acsassert.FileExists anti-patterns removed [code]
```bash
grep -c "acsassert\.FileExists" acs/cycle43/predicates_test.go
# Expected: 0 (all replaced with fixtures.FilePresent)
```

### AC-2: cycle99 acsassert.FileExists anti-patterns removed [code]
```bash
grep -c "acsassert\.FileExists" acs/cycle99/predicates_test.go
# Expected: 0 (all replaced with fixtures.FilePresent)
```

### AC-3: cycle106 stale t.Skip removed [code]
```bash
grep -c 'stale cycle106 version pin check skipped' acs/cycle106/predicates_test.go
# Expected: 0
```

### AC-4: cycle106 version check is format-agnostic (no "12.1.1" hardcode) [code]
```bash
grep -c '"12\.1\.1"' acs/cycle106/predicates_test.go
# Expected: 0
```

### AC-5: cycle106 binary path updated to repo root [code]
```bash
grep -c 'filepath.Join(root, "go", "evolve")' acs/cycle106/predicates_test.go
# Expected: 0 (stale path removed)
```

### AC-6: ACS cycle106 test suite compiles and passes [code]
```bash
cd go && go test ./acs/cycle106/... -count=1 -timeout 60s
# Expected: exit 0, no FAIL lines
```

### AC-7: ACS cycle43 test suite compiles and passes [code]
```bash
cd go && go test ./acs/cycle43/... -count=1 -timeout 60s 2>&1 | grep -E "^ok|FAIL"
# Expected: "ok" (not "FAIL")
```

### AC-8: ACS cycle99 test suite compiles and passes [code]
```bash
cd go && go test ./acs/cycle99/... -count=1 -timeout 60s 2>&1 | grep -E "^ok|FAIL"
# Expected: "ok" (not "FAIL")
```

### AC-9: fixtures import present in modified files [code]
```bash
# cycle43 and cycle99 must import test/fixtures for FilePresent
grep -l "fixtures.FilePresent" acs/cycle43/predicates_test.go acs/cycle99/predicates_test.go | wc -l
# Expected: 2
```

### NEGATIVE: Verify format-agnostic check catches RC suffix [code]
```bash
# The new version test should assert no "-rc" in output; simulate this
echo 'evolve 12.1.1-rc4 (abc)' | grep -v '\-rc' | wc -l
# Expected: 0 (RC suffix is caught)
```

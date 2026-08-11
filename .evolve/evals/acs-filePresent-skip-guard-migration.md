# Eval: acs-filepresent-skip-guard-migration

## Purpose
Verify that conditional `acsassert.FileExists(t, path)` guard uses in non-legacy ACS
predicate tests (cycle75, cycle77, cycle78, cycle102) are replaced with `fixtures.FilePresent(path)`,
and the stale permanent `t.Skip` in cycle106 `TestC106_011_BinaryVersionIsV12_1_1` is removed
and replaced with a timeless "no RC suffix" invariant.

## Scope
Non-legacy ACS packages: `go/acs/cycle75/`, `go/acs/cycle77/`, `go/acs/cycle78/`,
`go/acs/cycle102/`, `go/acs/cycle106/`

## Code Graders [code]

### AC-1: Build passes
`cd go && go build ./...`
Expected: exit 0.

### AC-2: Affected packages all pass
`cd go && go test ./acs/cycle75/... ./acs/cycle77/... ./acs/cycle78/... ./acs/cycle102/... ./acs/cycle106/... 2>&1 | grep -c "^FAIL"` — Expected output: `0`

### AC-3: No conditional FileExists guards remain in non-legacy packages
`grep -rn "acsassert\.FileExists" go/acs/cycle75/ go/acs/cycle77/ go/acs/cycle78/ go/acs/cycle102/`
Expected: no output (zero matches).

### AC-4: Stale permanent t.Skip removed from cycle106
`grep -n "stale cycle106 version pin check skipped" go/acs/cycle106/predicates_test.go`
Expected: no output.

### AC-5: cycle106 updated test checks "no RC suffix" invariant
`grep -in "strings.Contains\|strings.ToLower\|rc" go/acs/cycle106/predicates_test.go | grep -i "rc"`
Expected: at least one match confirming the RC-suffix check exists.

### AC-6: fixtures.FilePresent is a pure boolean (no error logging)
`grep -n "Errorf\|Fatal\|Helper\|testing\.TB\|testing\.T" go/test/fixtures/assert.go | grep -i "FilePresent"`
Expected: no matches — FilePresent must not call any TB methods.

### AC-7: Full test suite remains green
`cd go && go test ./... 2>&1 | grep "^FAIL" | head -5`
Expected: no output (zero FAIL lines).

## Negative / Edge Cases [code]

### NEG-1: Direct assertion uses of acsassert.FileExists preserved
`grep -rn "acsassert\.FileExists" go/acs/cycle42/predicates_test.go | wc -l`
Expected: `1` or more — real assertion uses must NOT be removed.

### NEG-2: Mutation kill — guard use would fail before migration
The original `if acsassert.FileExists(t, ref)` pattern calls `tb.Errorf` when ref is missing,
marking the test failed even before the inner conditional executes. After migration to
`fixtures.FilePresent(ref)`, absent optional files are silently skipped (no error logged).
To confirm the fix kills the mutant:
`grep -rn "func FilePresent" go/test/fixtures/assert.go | grep "bool"`
Expected: exactly one match with return type `bool` (no TB parameter), confirming FilePresent is pure.

## Acceptance Notes
- `fixtures.FilePresent` is defined in `go/test/fixtures/assert.go` — it takes only `path string` (no testing.TB)
- `acsassert.FileExists(t, path)` calls `tb.Errorf` when absent — valid for direct assertions, wrong for conditional guards
- Legacy packages (`//go:build legacy`: cycle43, cycle99, cycle101) are out of scope this cycle

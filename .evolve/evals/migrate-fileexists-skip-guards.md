# Eval: migrate-fileexists-skip-guards

## Task
Add `FilePresent(path string) bool` to `go/pkg/acsassert/assertions.go` (no `*testing.T`, pure boolean, no Errorf) and migrate the skip-guard sites in active (non-legacy) ACS cycle test files that check runtime-ephemeral or legacy-deleted files. The core false-red: `!acsassert.FileExists(t, f) { t.Skip(...) }` marks the test FAILED (via Errorf) before Skipping — so on a clean clone it appears RED. Using `!acsassert.FilePresent(path) { t.Skip(...) }` skips cleanly.

Target files containing `.evolve/runs/...` or `legacy/scripts/...` paths in their FileExists skip-guards:
- `go/acs/cycle57/predicates_test.go`
- `go/acs/cycle66/predicates_test.go`

## Acceptance Criteria

### AC1: FilePresent helper exists in acsassert [code]
```
grep -n "func FilePresent" /Users/danleemh/ai/claude/evolve-loop/go/pkg/acsassert/assertions.go
```
Expected: line showing `func FilePresent(path string) bool` (no `*testing.T`, no TB parameter)

### AC2: FilePresent has no TB/Errorf call [code]
```
grep -A5 "func FilePresent" /Users/danleemh/ai/claude/evolve-loop/go/pkg/acsassert/assertions.go
```
Expected: No `Errorf` or `tb.` in the function body — pure file-stat + bool return only

### AC3: Migrated skip-guards use FilePresent not FileExists [code]
```
grep "FileExists.*Skip\|Skip.*FileExists" /Users/danleemh/ai/claude/evolve-loop/go/acs/cycle57/predicates_test.go /Users/danleemh/ai/claude/evolve-loop/go/acs/cycle66/predicates_test.go 2>/dev/null | wc -l
```
Expected: `0` — no skip-guards in those two files still using FileExists

### AC4: Tests compile and pass [code]
```
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./pkg/acsassert/... ./acs/cycle57/... ./acs/cycle66/... 2>&1 | tail -5
```
Expected: `ok` lines for all packages, no FAIL

### AC5: Negative — FilePresent returns false for absent path (pure boolean, no panic) [code]
```
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./pkg/acsassert/... -run TestFilePresent -v 2>&1 | tail -10
```
Expected: A test `TestFilePresent*` exists and passes, proving the pure-boolean semantics work

# Eval: migrate-acs-skip-guards

## Goal
Replace `acsassert.FileExists(t, path)` used as skip-guards (where the test
calls t.Skip immediately after) with `fixtures.FilePresent(path)` in the
13 non-legacy acs predicates files. `acsassert.FileExists` calls `t.Errorf`
even when used as a skip precondition, turning file-absence into a FAIL
instead of a SKIP.

## Target files (non-legacy, run without -tags legacy)
go/acs/cycle42, cycle45, cycle57, cycle66, cycle72, cycle73, cycle74,
cycle75, cycle77, cycle78, cycle86, cycle102, cycle104

## Criteria

### C1 — Non-legacy acs suite passes with no FAIL [code]
```bash
cd go && go test -count=1 ./acs/... 2>&1
# Only cycle106 was previously failing; after migration, output must not
# contain "--- FAIL:" (cycle106 fix is a separate task).
# All non-cycle106 packages must be "ok".
```
Expected: all non-cycle106 packages report `ok`.

### C2 — acsassert.FileExists no longer appears in skip-guard context [code]
```bash
# Count occurrences of acsassert.FileExists immediately before a t.Skip call
grep -rn "acsassert.FileExists" go/acs/cycle42 go/acs/cycle45 go/acs/cycle57 \
  go/acs/cycle66 go/acs/cycle72 go/acs/cycle73 go/acs/cycle74 go/acs/cycle75 \
  go/acs/cycle77 go/acs/cycle78 go/acs/cycle86 go/acs/cycle102 go/acs/cycle104 \
  --include="*_test.go" 2>/dev/null | wc -l
```
Expected: output is `0` (all skip-guard sites migrated).

### C3 — fixtures.FilePresent is imported and used in migrated files [code]
```bash
grep -rn "fixtures.FilePresent\|FilePresent" go/acs/ --include="*_test.go" | wc -l
```
Expected: count > 0 (FilePresent calls present in the migrated files).

### C4 — No t.Errorf logged on file-absence (skip is clean) [code]
Run a migrated test against a deliberately absent path and verify SKIP not FAIL:
```bash
# cycle57 checks .evolve/runs/cycle-57/ artifacts that may not exist here.
cd go && go test -v -count=1 ./acs/cycle57/... 2>&1
# Must NOT contain "--- FAIL:" for cycle57
```
Expected: exit 0, `ok github.com/.../acs/cycle57`.

### NEG-1 — Negative: acsassert.FileExists in an assertion context is preserved [code]
```bash
# acsassert.FileExists used for positive assertions (not skip-guards) must remain.
# Example: cycle75 uses it to assert a file MUST exist (not to skip).
grep -n "acsassert.FileExists" go/acs/cycle75/predicates_test.go
```
Expected: if the remaining call is an assertion (not followed by t.Skip), the
output line does NOT immediately precede a `t.Skip` call.

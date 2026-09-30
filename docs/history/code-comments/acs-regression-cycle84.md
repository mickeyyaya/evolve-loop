# Comment history: `acs/regression/cycle84`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle84/predicates_test.go:3` — above `package cycle84`

```text
// Package cycle84 ports the cycle-84 ACS predicates (3 bash files).
```

### `go/acs/regression/cycle84/predicates_test.go:17` — above `func TestC84_001_LintBaselineExists(t *testing.T) {`

```text
// TestC84_001_LintBaselineExists ports cycle-84/001.
// .evolve/baselines/lint-markdown-structure-baseline.txt has >=10 lines.
```

### `go/acs/regression/cycle84/predicates_test.go:35` — above `func TestC84_002_CarryoverTodosSchemaValid(t *testing.T) {`

```text
// TestC84_002_CarryoverTodosSchemaValid ports cycle-84/002 (re-baselined
// 2026-06-05): state.json:carryoverTodos must be ABSENT or an ARRAY, and every
// entry must carry id/action/priority (go/internal/core/ports.go). The original
// "must be an empty array" assertion was retired — queueing deferred work via
// carryoverTodos[] is the sanctioned operator workflow, so a non-empty array is
// valid as long as it is well-formed. (The prior Go port both used the stale
// empty-array rule AND mis-used the asserting FileMatchesRegex helper, which
// t.Errorf'd before the intended t.Skipf could run — a false RED.)
```

### `go/acs/regression/cycle84/predicates_test.go:73` — above `func TestC84_003_ChangelogEntryExists(t *testing.T) {`

```text
// TestC84_003_ChangelogEntryExists ports cycle-84/003.
// CHANGELOG.md contains "Cycle 84" (case-insensitive).
```

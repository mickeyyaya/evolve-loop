# Comment history: `acs/regression/cycle50`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle50/predicates_test.go:3` — above `package cycle50`

```text
// Package cycle50 ports the cycle-50 ACS predicates (9 bash files).
```

### `go/acs/regression/cycle50/predicates_test.go:14` — above `func TestC50_001_ScoutStep45Exists(t *testing.T) {`

```text
// TestC50_001_ScoutStep45Exists ports cycle-50/001.
// evolve-scout.md has Step 4.5 + all six cache-check exit codes.
```

### `go/acs/regression/cycle50/predicates_test.go:35` — above `func TestC50_002_ScoutStep55Exists(t *testing.T) {`

```text
// TestC50_002_ScoutStep55Exists ports cycle-50/002.
// Soft-passes when the Step 5.5 section has been refactored away.
```

### `go/acs/regression/cycle50/predicates_test.go:48` — above `func TestC50_003_ScoutStopCriterionCacheSection(t *testing.T) {`

```text
// TestC50_003_ScoutStopCriterionCacheSection ports cycle-50/003.
```

### `go/acs/regression/cycle50/predicates_test.go:63` — above `func TestC50_004_BuilderStep25ResearchPointer(t *testing.T) {`

```text
// TestC50_004_BuilderStep25ResearchPointer ports cycle-50/004.
// Soft-passes when the research-pointer integration has been removed.
```

### `go/acs/regression/cycle50/predicates_test.go:76` — above `func TestC50_005_TriagePassthroughAllThreeFields(t *testing.T) {`

```text
// TestC50_005_TriagePassthroughAllThreeFields ports cycle-50/005.
```

### `go/acs/regression/cycle50/predicates_test.go:90` — above `func TestC50_006_ReconcileInvalidateOnDrop(t *testing.T) {`

```text
// TestC50_006_ReconcileInvalidateOnDrop ports cycle-50/006.
```

### `go/acs/regression/cycle50/predicates_test.go:104` — above `func TestC50_007_ReconcilePromoteOnPass(t *testing.T) {`

```text
// TestC50_007_ReconcilePromoteOnPass ports cycle-50/007.
```

### `go/acs/regression/cycle50/predicates_test.go:120` — above `func TestC50_008_InjectTaskResearchPointerFlag(t *testing.T) {`

```text
// TestC50_008_InjectTaskResearchPointerFlag ports cycle-50/008.
// Bash version actually runs inject-task.sh --dry-run; Go port asserts
// presence of the flag plumbing only. The bash predicate is authoritative
// for runtime behavior.
```

### `go/acs/regression/cycle50/predicates_test.go:138` — above `func TestC50_009_TesterDualVarWorktreePattern(t *testing.T) {`

```text
// TestC50_009_TesterDualVarWorktreePattern ports cycle-50/009.
```

# Comment history: `acs/regression/cycle98`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle98/predicates_test.go:3` — above `package cycle98`

```text
// Package cycle98 ports the cycle-98 ACS predicates (5 bash files).
// Subjects: triage phase-skip schema, orchestrator phase-skip precedence,
// phase-gate forward-skip-under-flag, no-role-execution invariant.
```

### `go/acs/regression/cycle98/predicates_test.go:16` — above `func TestC98_001_TriageSchemaDocumentsPhaseSkip(t *testing.T) {`

```text
// TestC98_001_TriageSchemaDocumentsPhaseSkip ports cycle-98/001.
```

### `go/acs/regression/cycle98/predicates_test.go:28` — above `func TestC98_002_OrchestratorHonorsPhaseSkipWithPrecedence(t *testing.T) {`

```text
// TestC98_002_OrchestratorHonorsPhaseSkipWithPrecedence ports cycle-98/002.
```

### `go/acs/regression/cycle98/predicates_test.go:40` — above `func TestC98_003_PhaseGateAcceptsForwardSkipUnderFlag(t *testing.T) {`

```text
// TestC98_003_PhaseGateAcceptsForwardSkipUnderFlag ports cycle-98/003.
```

### `go/acs/regression/cycle98/predicates_test.go:52` — above `func TestC98_004_PhaseSkippedImpliesNoRoleExecution(t *testing.T) {`

```text
// TestC98_004_PhaseSkippedImpliesNoRoleExecution ports cycle-98/004.
```

### `go/acs/regression/cycle98/predicates_test.go:64` — above `func TestC98_005_DefaultOffNoPhaseSkippedBaseline(t *testing.T) {`

```text
// TestC98_005_DefaultOffNoPhaseSkippedBaseline ports cycle-98/005.
```

### `go/acs/regression/cycle98/predicates_test.go:71` — above `if !acsassert.FileContainsAny(claudeMd, "EVOLVE_PSMAS_SKIP") {`

```text
// Soft check — only validate when the flag is documented in CLAUDE.md.
// CLAUDE.md may have evolved past the cycle-98 era and dropped the row.
```

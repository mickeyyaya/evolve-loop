# Comment history: `acs/regression/cycle77`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/cycle77/predicates_test.go:3` — above `package cycle77`

```text
// Package cycle77 ports the cycle-77 ACS predicates (1 bash file, 4 ACs).
```

### `go/acs/regression/cycle77/predicates_test.go:16` — above `func TestC77_001_AuditorColdMoveStage8(t *testing.T) {`

```text
// TestC77_001_AuditorColdMoveStage8 ports cycle-77/001.
// AC1: auditor persona ≤ 300 lines (≥10% reduction from 333)
// AC2: reference doc has "## Section: output-template"
// AC3: auditor pointer references reference/output-template
// AC4: ADR-0015 exists and ≤ 200 lines
```

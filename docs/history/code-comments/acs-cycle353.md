# Comment history: `acs/cycle353`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle353/predicates_test.go:3` — above `package cycle353`

```text
// Package cycle353 materializes the cycle-353 acceptance criteria for the
// committed top_n task:
//
//	observer-flag-classify — promote 4 Observer cluster flags from
//	StatusInternal to StatusActive in the flagregistry and fix the stale
//	EVOLVE_OBSERVER_NUDGE_S default in runtime-reference.md.
//
// AC map (1:1 with scout-report.md ACs):
//
//	AC1/AC3/AC5  observer spot-checks pass in flagregistry test suite  → C353_001
//	AC4          NUDGE_S default in runtime-reference.md is 300        → C353_002
//	AC6(neg)     STALL_S is no longer StatusInternal in registry_table  → C353_003
//	AC2          evolve flags check exits 0                            → C353_004 (pre-existing GREEN)
//
// Floor binding (R9.3): observer-flag-classify is the sole committed top_n
// task. No predicates for deferred items (cycle-280 lesson).
```

### `go/acs/cycle353/predicates_test.go:89` — above `func TestC353_003_ObserverStallSIsNotInternal(t *testing.T) {`

```text
// TestC353_003_ObserverStallSIsNotInternal is the NEGATIVE predicate (AC6):
// asserts that EVOLVE_OBSERVER_STALL_S is no longer annotated as StatusInternal
// in registry_table.go. Failure means the registry entry still reads
// "Status: StatusInternal" — the strongest anti-no-op signal for this task.
//
// Pair with C353_001: C353_001 verifies the flags ARE Active; C353_003 verifies
// they are NOT Internal. Together they prevent a vacuous implementation that
// just removes the flag entry without adding the Active row.
//
// RED: registry_table.go currently contains the string
// `EVOLVE_OBSERVER_STALL_S", Status: StatusInternal` because the flag was
// left as a placeholder from the 2026-06-11 inventory sweep.
// acs-predicate: config-check
```

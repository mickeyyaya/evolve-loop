# Comment history: `acs/cycle7`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle7/predicates_test.go:3` — above `package cycle7`

```text
// Package cycle7 materializes the cycle-7 acceptance criteria for the
// committed top_n task:
//
//   - add-ceiling-ratchet-retire-deprecated — build the FlagCeiling ratchet
//     gate at 258 and retire EVOLVE_FORCE_INNER_SANDBOX, EVOLVE_INNER_SANDBOX,
//     EVOLVE_PROFILE_WORKTREE_AWARE, and EVOLVE_REINVOKE_CMD from the registry
//     (262 → 258), regenerate control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	add-ceiling-ratchet-retire-deprecated:
//	  AC1+NEG1 FlagCeiling ratchet: len(All) ≤ 258               → C7_001 (behavioral, ceiling gate)
//	  AC3      Registry row count == 258                           → C7_002 (behavioral, exact count)
//	  AC4a     FORCE_INNER_SANDBOX absent from Lookup              → C7_003 (behavioral, absence)
//	  AC4b     INNER_SANDBOX absent from Lookup                    → C7_004 (behavioral, absence)
//	  AC4c     PROFILE_WORKTREE_AWARE absent from Lookup           → C7_005 (behavioral, absence)
//	  AC4d     REINVOKE_CMD absent from Lookup                     → C7_006 (behavioral, absence)
//	  EDGE1    control-flags.md: 0 stale entries for 4 retired    → C7_007 (config-check, waiver)
//
// AC2 (TestRegistry_FlagCeiling exists) and NEG2 (guard asserts ABSENT) are
// verified by the flagregistry test suite (AC5), which is covered by the
// repository's normal CI run. AC6 (cycle7 predicates pass) is self-referential.
// AC7 (full suite green) is enforced by CI; no duplicate predicate needed.
//
// Floor binding (R9.3): predicates only for committed top_n task
// (add-ceiling-ratchet-retire-deprecated). Deferred tasks get zero predicates.
```

### `go/acs/cycle7/predicates_test.go:129` — above `func TestC7_007_ControlFlagsDocHasNoStaleEntries(t *testing.T) {`

```text
// TestC7_007_ControlFlagsDocHasNoStaleEntries verifies that the generated
// docs/architecture/control-flags.md no longer lists any of the 4 retired
// flags after Builder removes their rows and regenerates the doc.
//
// // acs-predicate: config-check — the doc entries are generated from the
// registry; their absence follows from AC4 (rows removed). This predicate
// ensures the regeneration step (evolve flags generate) also ran.
//
// RED: control-flags.md currently lists all 4 deprecated flags (26d530e5 HEAD).
```

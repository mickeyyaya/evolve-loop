# Comment history: `acs/cycle14`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero e2

### `go/acs/cycle14/predicates_test.go:3` — above `package cycle14`

```text
// Package cycle14 materializes the cycle-14 acceptance criteria for two tasks:
//
//   - ipcenv-leaf: Create IPC-const leaf package (internal/ipcenv) with 3 exported
//     consts: FleetKey, FleetScopeKey, WorktreeRootKey — each marked SSOT IPC-protocol-allowed.
//     Zero imports from this codebase (pure const leaf). Enrolled in .apicover-enforce.
//
//   - ipcenv-wire: Route all readers through ipcenv + remove 3 registry rows.
//     No bare "EVOLVE_FLEET", "EVOLVE_FLEET_SCOPE", or "EVOLVE_WORKTREE_ROOT" string
//     literals in production Go files (outside ipcenv/). fleet.go local consts removed.
//     flagregistry rows for the 3 flags deleted. Build clean.
//
// AC map (1:1 with scout-report.md ACs, all in top_n):
//
//	ipcenv-leaf:
//	  AC1  3 exported consts exist with correct values + SSOT comment → C14_001
//	  AC2  Package compiles (go build ./internal/ipcenv/...)           → C14_002
//	  AC3  Enrolled in .apicover-enforce                               → C14_003
//	  AC4  No imports from this codebase (leaf purity)                 → C14_004
//
//	ipcenv-wire:
//	  AC5  No bare EVOLVE_FLEET literal in prod files                  → C14_005
//	  AC5- SSOT in ipcenv.go DOES contain literal (grep-exclusion check) → C14_005neg
//	  AC6  No bare EVOLVE_FLEET_SCOPE literal in prod files            → C14_006
//	  AC7  No bare EVOLVE_WORKTREE_ROOT literal in prod files          → C14_007
//	  AC8  3 registry rows deleted                                     → C14_008
//	  AC8- An unrelated flag still in registry (surgical-delete check) → C14_008neg
//	  AC9  Build is clean after wiring (go build ./...)                → C14_009
//	  AC10 fleet.go local consts removed                               → C14_010
//
// Floor binding (R9.3): predicates only for tasks in triage top_n.
```

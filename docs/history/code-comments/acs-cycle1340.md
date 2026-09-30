# Comment history: `acs/cycle1340`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1340/predicates_test.go:3` — above `package cycle1340`

```text
// Package cycle1340 materializes the acceptance criteria of this lane's sole
// top_n task, `defect-ledger-worktree-evidence-fallback` (triage-report.md
// "## top_n"; scout-report.md Task 1 + Task 2, folded into one card by triage
// because the fix and its regression proof share one worktree/build/audit).
//
// The defect: `evidenceResolves` (go/internal/phases/audit/defect_ledger.go:253)
// resolves every closure citation with ONE os.Lstat under req.ProjectRoot. A
// continuation lane's own fix lives in its own still-open worktree and reaches
// the project root only on merge — which is exactly what this gate blocks.
// Cycles 1320 → 1323 → 1325 → 1330 each cited the same two real files and were
// rejected four times with the identical "resolves to no file under the project
// root". P0, cycles_unpicked=5+.
//
// EXECUTION SHAPE (why these predicates shell one narrowed `go test`):
// the subject, `evidenceResolves`, is unexported, and its only production
// caller is `reconcileAgainstAncestor` ← `reconcileContinuationDefects` ←
// `hooks.Classify` — all unexported inside package `audit`. An external acs
// package cannot reach that seam, and re-implementing the gate here would
// assert on a copy. So the behavioral contract lives IN the package
// (internal/phases/audit/defect_ledger_worktree_evidence_test.go), driven
// through the REAL production seam hooks{}.Classify, and each predicate below
// runs ONE named test in ONE named package with `-run` narrowing —
// the sanctioned shape (cycle1300/cycle1310/cycle1323 precedent), not a
// `./...` sweep and not one of the known 40s+ suites.
//
// A zero exit is NOT sufficient: each predicate demands the `--- PASS: <name>`
// receipt, so a deleted or skipped test reads as a miss rather than a pass —
// the frozen contract cannot be satisfied by removing it (rule 4).
//
// No new go/internal package is created by this task, so ADR-0069's repo-wide
// apicover enrollment does not apply (nothing to append to go/.apicover-enforce,
// no apicover_named_test.go to author).
//
// Adversarial diversity:
//
//	C1340_001 positive  — worktree-resident evidence closes an inherited defect
//	                      (the 1320→1330 deadlock, broken).
//	C1340_002 negative  — evidence under NEITHER root still blocks PASS
//	                      (deleting the Lstat passes 001 and fails this).
//	C1340_003 negative  — the self-citation guard (rule 4) rejects the gate's own
//	                      bookkeeping under the WORKTREE root too — the graded
//	                      agent writes that tree, so this is the fix's cheapest
//	                      bypass; plus the '..' escape rejection with a worktree
//	                      root present.
//	C1340_004 edge/regr — project-root evidence still closes when a worktree is
//	                      also set (fallback, not replacement), and an empty
//	                      req.Worktree (provisioning failed) still blocks.
//	C1340_005 no-regr   — the whole Classify verdict family in the audit package
//	                      stays green; the gate is shared by every cycle.
```

### `go/acs/cycle1340/predicates_test.go:131` — above `func TestC1340_003_worktree_root_is_not_a_bypass(t *testing.T) {`

```text
// TestC1340_003_worktree_root_is_not_a_bypass — NEGATIVE. AC3: rule 4 (the
// self-citation guard, cycle-1285 F3) and the '..'-escape rejection apply to
// the worktree root exactly as to the project root. The graded agent writes
// its own worktree, so a guard that checks only ProjectRoot would hand the fix
// its cheapest bypass.
```

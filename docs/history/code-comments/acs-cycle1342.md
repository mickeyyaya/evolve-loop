# Comment history: `acs/cycle1342`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1342/predicates_test.go:3` — above `package cycle1342`

```text
// Package cycle1342 materializes the acceptance criteria of this lane's
// three top_n tasks (scout-report.md, fleet_scope
// `defect-ledger-worktree-evidence-fallback`):
//
//   - Task 1 auditor-disposition-schema-producer: agents/evolve-auditor.md
//     documents the defect-dispositions.json schema (F3, 0.97 — the actual
//     unblock for THIS cycle; a prompt gap, not a code gap).
//   - Task 2 defect-ledger-worktree-fallback-land: evidenceResolves
//     (go/internal/phases/audit/defect_ledger.go) resolves a closure
//     citation under req.Worktree when req.ProjectRoot misses, and treats a
//     ":line-line" range suffix as one locator. SELF-GRADING — this ports
//     the cycle-1340 fix so it grades SUCCESSOR cycles; it cannot change
//     THIS cycle's verdict (see build-report.md).
//   - Task 3 disposition-completeness-preflight: a continuation whose
//     defect-dispositions.json is entirely absent, or covers fewer ids than
//     the ancestor ledger enumerates, must fail with a diagnostic named
//     distinctly from the existing per-id "(no disposition)" switch branch
//     — a structural pre-flight, not only a per-id fallthrough.
//
// EXECUTION SHAPE (why these predicates shell one narrowed `go test`):
// the Task 2/3 subjects (evidenceResolves, the disposition pre-flight) are
// unexported, and their only production caller is hooks.Classify — all
// inside package `audit`. An external acs package cannot reach that seam,
// and re-implementing the gate here would assert on a copy. So the
// behavioral contract lives IN the package
// (internal/phases/audit/defect_ledger_worktree_evidence_test.go and
// internal/phases/audit/disposition_preflight_test.go), driven through the
// REAL production seam hooks{}.Classify, and each predicate below runs ONE
// named test in ONE named package with `-run` narrowing — never a `./...`
// sweep and not one of the known 40s+ suites (flaky-predicate-shape rule 1).
//
// A zero exit is NOT sufficient: each predicate demands the `--- PASS:
// <name>` receipt, so a deleted or skipped test reads as a miss rather than
// a pass — the frozen contract cannot be satisfied by removing it.
//
// No new go/internal package is created by any of the three tasks, so
// ADR-0069's repo-wide apicover enrollment does not apply.
//
// Adversarial diversity:
//
//	C1342_001 positive  — agents/evolve-auditor.md names the
//	                      defect-dispositions.json schema + re-author rule.
//	C1342_002 positive  — worktree-resident evidence closes an inherited
//	                      defect (the 1320→1330 deadlock, broken).
//	C1342_003 negative  — evidence under NEITHER root still blocks PASS.
//	C1342_004 negative  — the self-citation guard + '..'-escape rejection
//	                      survive the worktree fallback.
//	C1342_005 edge/regr — project-root evidence unchanged; empty
//	                      req.Worktree still blocks; ":line-line" ranges
//	                      resolve; a non-numeric ':' suffix is NOT stripped.
//	C1342_006 no-regr   — the whole Classify verdict family stays green.
//	C1342_007 negative  — a continuation with NO defect-dispositions.json at
//	                      all fails with a NAMED pre-flight diagnostic.
//	C1342_008 negative  — a continuation whose defect-dispositions.json
//	                      covers only SOME inherited ids fails with a NAMED
//	                      pre-flight diagnostic naming what's missing.
//	C1342_009 edge       — a fully-dispositioned continuation trips NEITHER
//	                      new pre-flight message (anti-no-op: a pre-flight
//	                      that always fires proves nothing); an ordinary
//	                      (non-continuation) cycle trips neither either.
```

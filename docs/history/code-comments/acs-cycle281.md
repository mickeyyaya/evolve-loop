# Comment history: `acs/cycle281`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle281/predicates_test.go:3` — above `package cycle281`

```text
// Package cycle281 materializes the cycle-281 acceptance criteria for the three
// committed top_n tasks (scout-report.md — coverage + adversarial-testing
// campaign):
//
//	T1  fix-inserted-phase-worktree-dispatch — the cycle-280 P0: advisor-INSERTED
//	    write-capable phases dispatched with Worktree="" (mint default
//	    writes_source:false) → tree-diff guard cycle-fatal + abort-cleanup deletes
//	    the uncommitted worktree. Fix: a minted phase defaults to write-capable
//	    (inherits the worktree); an explicit writes_source:false stays read-only;
//	    an abnormal mid-cycle abort PRESERVES the worktree.
//	T2  adversarial-fault-injection-suite    — a real fault-injection suite (not the
//	    cycle-280 schema-only skeleton) driving fakeTmux scripted panes across the
//	    six fault types (stall / crash / update-menu / weak-busy / empty-pane /
//	    malformed) × the three driver families (claude / codex / agy).
//	T3  coverage-push-core-and-lower-packages — lift internal/core and
//	    internal/routingtest to >= 90% with intent-probing tests.
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test as a real subprocess — `go test -v` over the core /
// bridge packages and `go tool cover` totals — and assert on the real
// `--- PASS: <name>` lines, sub-case counts, and coverage numbers the builder's
// tests produce. A magic string in a .go file can neither produce a named PASS
// line nor move a coverage number, so none of these is gameable by source editing
// alone (the established cycle-274/276 pattern).
//
// Convention: the TDD-engineer authors T1's three failing tests
// (orchestrator_inserted_worktree_test.go); the BUILDER authors T2's fault suite
// (adversarial_faults_test.go + its TestAdversarialFaultMatrix_* guards) and T3's
// coverage tests. These ACS predicates GATE on those tests running + passing with
// the required adversarial diversity. RED at baseline: T1 has two failing tests,
// T2's suite does not exist, and the coverage numbers sit below the floors.
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.a TestInsertedPhaseWritableInheritsWorktree PASS  → C281_001
//	T1.b TestAbortCleanupPreservesWorktreeDiff PASS       → C281_001
//	T1.c TestInsertedReadOnlyPhaseDoesNotGetWorktree PASS → C281_001 (discriminator)
//	T2.a adversarial suite >= 18 passing cases            → C281_010
//	T2.b 6 fault types present as passing cases           → C281_011
//	T2.c 3 driver families present as passing cases       → C281_012
//	T2.d TestAdversarialFaultMatrix_* guards PASS         → C281_013
//	T3.a internal/core coverage >= 90%                    → C281_020
//	T3.b internal/routingtest coverage >= 90%             → C281_021
```

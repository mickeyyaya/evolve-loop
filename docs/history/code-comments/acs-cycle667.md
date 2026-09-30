# Comment history: `acs/cycle667`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle667/predicates_test.go:3` — above `package cycle667`

```text
// Package cycle667 materialises the cycle-667 acceptance criteria for the single
// triage-committed (`## top_n`) task: chronicle-s4-carryover-orphan-merge (weight
// 0.92).
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-decision.json / triage-report.md commit exactly ONE task to this cycle:
//	  chronicle-s4-carryover-orphan-merge — C667_001..007
//	The scout report proposed the echo-veto-wiring family, but triage `## top_n`
//	(the sole task authority) DEFERRED those and committed chronicle-s4 instead, so
//	the predicates bind to THAT. Every non-committed / deferred item gets ZERO
//	predicates.
//
// ORPHAN CONTEXT — evolve-memo (the PASS-branch scribe, dispatched post-ship)
// writes <workspace>/carryover-todos.json every PASS cycle, and the retro path
// writes the same file on FAIL, but NO Go code ever reads it (grep: zero
// readers). The PASS-branch learning channel is fire-and-forget: queued todos
// never reach state.json:carryoverTodos, so the next cycle's planner never sees
// them. Fixed this cycle by a cycle-terminal hook (MergeWorkspaceCarryover, wired
// in finalizeCycle beside persistCycleEndState) that tolerant-decodes the file,
// caps + priority-maps + TTL-stamps each entry, and merges via the existing
// mergeCarryoverTodos (dedup by id ⇒ idempotent).
//
// PREDICATE QUALITY (cycle-85): every predicate EXERCISES the SUT. Each shells
// `go test -race -v -run <name>` against the real internal/core package and
// asserts the named TDD-authored behavioral test actually RAN and PASSED (the
// `--- PASS: <name>` marker) — a package that compiles but lacks the test prints
// "no tests to run" (exit 0) with NO marker, so a bare exit check would vacuously
// green. C667_006 additionally asserts the touched package builds against the new
// surface AND that no DATA RACE is reported.
//
// TEST-NAME CONTRACT — these behavioral tests are authored by the TDD engineer in
// go/internal/core/carryover_merge_test.go (RED now; Builder must NOT modify them,
// only add production code — go/internal/core/carryover_merge.go + the
// finalizeCycle call site — to green them):
//
//	internal/core : TestRunCycle_MergesMemoCarryoverTodosIntoState
//	                TestMergeWorkspaceCarryover_DedupesById
//	                TestMergeWorkspaceCarryover_CapsActionRunes
//	                TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails
//	                TestMergeWorkspaceCarryover_StampsExpiryForPrune
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive/wiring : C667_001 the real terminal path (finalizeCycle) persists
//     the memo todos — the load-bearing anti-no-op signal (a defined-but-unwired
//     helper leaves the orphan open and fails here).
//   - Semantic        : C667_002 re-entry is idempotent (dedup by id).
//   - Edge            : C667_003 an oversized action is capped.
//   - Negative/OOD    : C667_004 malformed JSON + id/action-less entries tolerated
//     (WARN, no panic, no fatal).
//   - Semantic        : C667_005 every merged todo carries a future ExpiresAt so
//     the loop-start prune converges.
//   - Integrity       : C667_006 internal/core builds against the new surface and
//     the merge tests pass under -race (no DATA RACE).
```

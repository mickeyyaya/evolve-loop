# Comment history: `acs/cycle433`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle433/predicates_test.go:3` — above `package cycle433`

```text
// Package cycle433 materialises the cycle-433 acceptance criteria for slice
// S5 of the SignalCenter consolidation campaign (goal:
// aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd).
//
// S5 is the campaign's final slice: harden SignalCenter's concurrency model
// under ParallelEvaluate-style dispatch and resolve ADR-0068's "Deferred
// (S5)" per-session sharding decision with measured evidence.
//
// Tasks (Task B dependsOn Task A):
//
//	s5-parallelevaluate-stress-race (Task A — S, P0, dependsOn: none):
//	  A mixed-op stress harness (≥16 producers × ≥100 Observe cycles, plus
//	  concurrent Aggregate/Busy/Changed readers and concurrent
//	  RegisterHandler) on ONE shared *SignalCenter, race-clean, asserting the
//	  Aggregate() invariant on every read. Written against the ALREADY-SHIPPED
//	  SignalCenter (S2-S4, on main) — no production change is required, so
//	  these predicates run GREEN today (pre-existing GREEN; they PIN the
//	  concurrency invariant for Task B's evidence-driven refactor).
//
//	s5-resolve-sharding-decision (Task B — M, P0, dependsOn Task A):
//	  Add BenchmarkSignalCenter_ParallelObserve (distinct session keys); use
//	  its measured result to RESOLVE ADR-0068's deferred sharding decision
//	  (implement minimal per-session sharding OR record
//	  single-mutex-sufficient); document the concurrency model (ownership,
//	  lock ordering, no-data-race invariant). RED today: the ADR still
//	  contains the literal "Deferred (S5)" string, has no lock-ordering
//	  phrase, and no BenchmarkSignalCenter_ParallelObserve exists anywhere in
//	  the panestream package.
//
// AC map (1:1 against the full graders in
// .evolve/evals/s5-parallelevaluate-stress-race.md and
// .evolve/evals/s5-resolve-sharding-decision.md; predicates for ## top_n
// tasks only, R9.3 floor-binding):
//
//	Task A (s5-parallelevaluate-stress-race):
//	  AC1 stress test exists, ≥16 producers × ≥100 cycles, mixed overlapping
//	      ops (positive)                                     → C433_001 pre-existing GREEN
//	  AC2 race-clean under -race (regression)                → C433_001 (same run, -race flag) pre-existing GREEN
//	  AC3 real guard: exercises ALL FIVE ops, not a cheap
//	      distinct-keys-only fake (anti-gaming)               → C433_003 pre-existing GREEN
//	  AC4 Aggregate never invalid under concurrency, incl. the
//	      same-key torn-read shape (BA2) (edge/negative)      → C433_001 + C433_002 pre-existing GREEN
//	  AC5 apicover clean, no new exported symbol (regression) → C433_004 pre-existing GREEN
//
//	Task B (s5-resolve-sharding-decision):
//	  AC1 distinct-key Observe benchmark exists and runs      → C433_007 RED today (no such func exists)
//	  AC2 ADR-0068 no longer says "Deferred (S5)" (negative)  → C433_005 RED today
//	  AC3 ADR documents concurrency model: ownership + lock
//	      ordering + no-data-race invariant (positive)        → C433_006 RED today
//	  AC4 IF sharded: Observe on distinct keys no longer holds
//	      a single process-global lock across Assess(); lock
//	      ordering documented+enforced; Aggregate/Busy/Changed
//	      read under the SAME lock Observe writes under (no
//	      torn read) — contingent on a decision not yet made   → manual+checklist (see test-report.md; C433_002/C433_008 are the automated backstop regardless of which branch is chosen)
//	  AC5 public API unchanged; no new exported symbol without
//	      an AST-naming test (regression)                     → C433_004 + C433_008 pre-existing GREEN
//
// RED strategy: C433_001-004 are pre-existing GREEN (Task A needs zero
// production change — the current single-RWMutex model, S2-S4, already
// satisfies it). C433_005-007 are genuinely RED today, confirmed by direct
// rerun immediately before test-report.md was written. C433_008 (full-suite
// regression) is pre-existing GREEN and MUST stay green after Task B lands.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C433_002 (a torn read on the SAME key, not merely disjoint
//	            keys — the shape that would surface BA2), C433_003 (a test
//	            that only drives Observe on distinct keys — the cheapest
//	            possible fake — FAILS this grader; forces genuine op
//	            diversity across all five SignalCenter operations)
//	Edge/OOD:   C433_001 re-runs the pre-existing
//	            TestSignalCenter_EmptyCenter_DefinedState /
//	            TestSignalCenter_UnknownKeyIsQuiet under the SAME -race
//	            harness invocation (empty-center / unknown-key reads must
//	            stay defined under concurrency, not merely standalone)
//	Semantic:   C433_005 (doc text: deferral removed) vs. C433_006 (doc text:
//	            model documented) are DISTINCT textual claims, not one
//	            assertion restated — an ADR edit that removes "Deferred (S5)"
//	            without adding the lock-ordering/ownership documentation
//	            passes C433_005 but still fails C433_006
//
// 1:1 enforcement: Task A predicate=5 (AC1-5) → total=5 ✓ (matches the eval
// file's 5-AC list). Task B predicate=4 (AC1,AC2,AC3,AC5) + manual+checklist=1
// (AC4) → total=5 ✓ (matches the eval file's 5-AC list). Combined: 10/10 ACs
// dispositioned, zero bare "defer to Auditor" entries.
```

### `go/acs/cycle433/predicates_test.go:172` — above `func TestC433_004_ApicoverEnforceCleanBothPackages(t *testing.T) {`

```text
// TestC433_004_ApicoverEnforceCleanBothPackages (AC5, regression,
// pre-existing GREEN): apicover -enforce must report 0 uncovered / 0
// false-green symbols on both touched packages — the recurring
// cycles-413/426/430 CI-break class. Mirrors
// go/acs/cycle431/predicates_test.go's TestC431_004 exactly.
```

### `go/acs/cycle433/predicates_test.go:228` — above `func TestC433_005_ADRNoLongerDeferred(t *testing.T) {`

```text
// TestC433_005_ADRNoLongerDeferred (AC2, negative, RED today): ADR-0068 must
// no longer contain the literal "Deferred (S5)" string. FileNotContains
// (not an inverted FileContains — see go/acs/README.md's cycle-352 lesson)
// passes silently when the string is absent and fails when it is present.
```

### `go/acs/cycle433/predicates_test.go:236` — above `func TestC433_006_ADRDocumentsConcurrencyModel(t *testing.T) {`

```text
// TestC433_006_ADRDocumentsConcurrencyModel (AC3, positive, RED today):
// ADR-0068 must document BOTH the lock-ordering rule AND ownership/mutation
// semantics — two distinct textual claims (see Semantic diversity note
// above), not one assertion restated.
```

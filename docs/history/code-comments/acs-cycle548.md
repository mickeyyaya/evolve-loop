# Comment history: `acs/cycle548`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle548/predicates_test.go:3` — above `package cycle548`

```text
// Package cycle548 materialises the cycle-548 acceptance criteria for the single
// triage-committed (`## top_n`) task: loop-self-prioritize-unmet-fleet-concurrency.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this lane (blocker-solo,
//	Principle 5): loop-self-prioritize-unmet-fleet-concurrency. Every `## deferred`
//	item (fix-memo-phase-routing-gap, acsrunner-coverage-tag-parity,
//	fleet-min-width-lane-expansion, auditor-egps-reconciliation-gate, and the rest
//	of the out-of-scope backlog) gets ZERO predicates here.
//
// REFRAMING (fault-localization-report.md — authoritative, ran after triage):
//
//	The committed task's LITERAL prose ("add a config-driven concurrency-utilization
//	observer in go/internal/fleethealth + wiring") describes a feature that was
//	already designed, implemented, tested, and shipped in cycle 544 (commit
//	3a899218) as go/internal/fleet/starvation.go. EVERY bullet of the stale inbox
//	item's own "acceptance" array is already satisfied at HEAD (StarvationTracker /
//	WaveObservation.Starved() / BuildStarvationItem / WriteTo). Rebuilding it — and
//	especially resurrecting the internal/fleethealth leaf — is a NO-OP that would
//	FAIL the cycle-544 anti-regression predicate and re-introduce the cycle-542
//	apicover-graduation defect.
//
//	The REAL fault is a process / data-integrity gap in the inbox lifecycle:
//	go/internal/phases/ship/postship.go promoteInbox/extractIDs retires ONLY inbox
//	ids that match the SHIPPING cycle's own top_n[].id ∪ skip_shipped[].task_id.
//	Cycle 544 shipped the capability under the synthesized id
//	"recover-ship-fleet-starvation-observer", so the ORIGINAL inbox item
//	"loop-self-prioritize-unmet-fleet-concurrency" was never retired and scout/triage
//	keep re-selecting already-completed work (cycles 545..548). The durable fix is a
//	reconciliation seam that retires an inbox item by id ALONE — even when that id
//	differs from the shipping cycle's committed work.
//
// DESIGN CONTRACT (what Builder must implement to green these — see test-report.md
// AC-Materialization for the 1:1 disposition and the reasoning behind picking a
// declared `superseded[]` list over "evaluate acceptance vs HEAD"; the latter is
// undecidable here because the stale item's acceptance bullets are PROSE, not
// runnable commands):
//
//	Two NEW exported symbols in the stdlib-only leaf go/internal/inboxmover
//	(the seam fault-localization suspect #3 names — "no new mover primitive, only a
//	new call site"; kept in inboxmover, NOT phases/ship, so these predicates stay
//	hermetic — no git, no heavyweight ship suite, the cycle-543 defect C544_003
//	guards against):
//
//	  func SupersededInboxIDs(triageDecisionJSON []byte) []string
//	    // deduped, order-preserving ids from the top-level "superseded" array;
//	    // nil on absent/invalid — never panics.
//	  func ReconcileSuperseded(opts Options, supersededIDs []string,
//	                           newState string, p PromoteOpts) ([]string, error)
//	    // retires (Promote → newState) each inbox item whose .id ∈ supersededIDs,
//	    // by id ALONE; ids not present are a clean idempotent no-op; returns the
//	    // ids actually retired.
//
//	postship.go promoteInbox then calls ReconcileSuperseded(opts,
//	SupersededInboxIDs(body), "processed", ...) alongside the existing top_n
//	promote (wiring dispositioned manual+checklist — a full class=cycle ship in a
//	predicate is the heavyweight-suite-in-a-predicate smell this repo forbids).
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it CALLS the real inboxmover.ReconcileSuperseded / SupersededInboxIDs and asserts
// on the returned ids AND the real file-move side effect (item gone from inbox
// root, present under processed/). None shells a foreign heavyweight suite. The
// single structural check (C548_004 dir-absence/placement) carries an explicit
// config-check waiver.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C548_001 a differently-named inbox item IS retired by id alone.
//   - Negative : C548_002 an UNDECLARED live inbox item is LEFT IN PLACE (the
//     strongest anti-no-op: a "retire everything" or "retire nothing" impl fails).
//   - Semantic : C548_003 the declaration parser dedups/order-preserves and is
//     empty (never panics) on an absent field or invalid JSON.
//   - Regression: C548_004 the already-shipped observer is NOT rebuilt — no
//     internal/fleethealth leaf, starvation.go still in internal/fleet (cycle-542).
//   - Edge      : C548_005 an absent id and an empty list are clean no-ops, not
//     errors (idempotent re-run safety).
```

### `go/acs/cycle548/predicates_test.go:104` — above `func TestC548_001_ReconcileSuperseded_RetiresDifferentlyNamedInboxItem(t *testing.T) {`

```text
// C548_001 — AC-1 (positive core, durable seam). ReconcileSuperseded retires an
// inbox item BY ID ALONE, even though its id differs from any shipping-cycle
// top_n/skip_shipped id — the exact orphan class that stranded
// "loop-self-prioritize-unmet-fleet-concurrency" across cycles 544..548. Exercises
// the real SUT and asserts BOTH the returned id set and the file-move side effect
// (gone from the live inbox root, present under processed/cycle-<N>/).
```

### `go/acs/cycle548/predicates_test.go:195` — above `func TestC548_004_ObserverNotRebuilt_NoFleethealthLeaf(t *testing.T) {`

```text
// C548_004 — AC-4 (structural anti-regression, config-check). The committed
// task's prose invites rebuilding the observer in internal/fleethealth — the exact
// cycle-542 anti-pattern the apicover completeness gate rejected. This guard
// asserts (1) the observer symbol is reachable FROM package fleet (compile-time
// fleet.StarvationTracker reference — a fleethealth-housed impl would not satisfy
// this import), (2) no internal/fleethealth directory exists, and (3)
// internal/fleet/starvation.go is still present (the observer must be REUSED, not
// rebuilt or relocated). Package-placement invariant, hence the config-check waiver.
//
// acs-predicate: config-check
```

//go:build acs

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
package cycle548

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// writeInboxItem drops a minimal inbox JSON (only .id is load-bearing for the
// mover, which keys off it) at the given path and fails the test on error.
func writeInboxItem(t *testing.T, path, id string) {
	t.Helper()
	body := []byte(`{"id":` + jsonQuote(id) + `,"weight":0.9,"kind":"feature"}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write inbox item %s: %v", path, err)
	}
}

// jsonQuote is a tiny local string quoter so the fixture stays dependency-free.
func jsonQuote(s string) string { return `"` + s + `"` }

// C548_001 — AC-1 (positive core, durable seam). ReconcileSuperseded retires an
// inbox item BY ID ALONE, even though its id differs from any shipping-cycle
// top_n/skip_shipped id — the exact orphan class that stranded
// "loop-self-prioritize-unmet-fleet-concurrency" across cycles 544..548. Exercises
// the real SUT and asserts BOTH the returned id set and the file-move side effect
// (gone from the live inbox root, present under processed/cycle-<N>/).
func TestC548_001_ReconcileSuperseded_RetiresDifferentlyNamedInboxItem(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	const id = "loop-self-prioritize-unmet-fleet-concurrency"
	const base = "2026-07-05T04-15-00Z-loop-self-prioritize-unmet-fleet-concurrency.json"
	writeInboxItem(t, filepath.Join(inbox, base), id)

	opts := inboxmover.Options{ProjectRoot: root}
	retired, err := inboxmover.ReconcileSuperseded(opts, []string{id}, "processed", inboxmover.PromoteOpts{Cycle: "548"})
	if err != nil {
		t.Fatalf("ReconcileSuperseded returned error: %v", err)
	}
	if len(retired) != 1 || retired[0] != id {
		t.Fatalf("retired = %v, want exactly [%q] (the superseded id retired by id alone)", retired, id)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, base)); !os.IsNotExist(statErr) {
		t.Fatalf("superseded item still present in the live inbox root (stat err=%v) — it must be retired", statErr)
	}
	dest := filepath.Join(inbox, "processed", "cycle-548", base)
	if _, statErr := os.Stat(dest); statErr != nil {
		t.Fatalf("superseded item not retired to %s: %v", dest, statErr)
	}
}

// C548_002 — AC-2 (negative / anti-no-op, selectivity). Reconciliation retires
// ONLY the declared superseded ids; every other live inbox item stays put. A
// degenerate implementation that drains the whole inbox (or one that moves
// nothing) FAILS here — this is the strongest anti-no-op signal for the seam.
func TestC548_002_ReconcileSuperseded_LeavesUndeclaredItemsInPlace(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	const retireBase = "2026-07-05T00-00-00Z-retire-me.json"
	const keepBase = "2026-07-06T00-00-00Z-keep-me.json"
	writeInboxItem(t, filepath.Join(inbox, retireBase), "retire-me")
	writeInboxItem(t, filepath.Join(inbox, keepBase), "keep-me")

	opts := inboxmover.Options{ProjectRoot: root}
	retired, err := inboxmover.ReconcileSuperseded(opts, []string{"retire-me"}, "processed", inboxmover.PromoteOpts{Cycle: "548"})
	if err != nil {
		t.Fatalf("ReconcileSuperseded returned error: %v", err)
	}
	if len(retired) != 1 || retired[0] != "retire-me" {
		t.Fatalf("retired = %v, want exactly [\"retire-me\"] — undeclared ids must not be retired", retired)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, keepBase)); statErr != nil {
		t.Fatalf("UNDECLARED item keep-me was removed from the inbox root (%v) — reconciliation must be selective, never a blanket drain", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, retireBase)); !os.IsNotExist(statErr) {
		t.Fatalf("declared item retire-me still in the inbox root (stat err=%v) — it must be retired", statErr)
	}
}

// C548_003 — AC-3 (semantic, declaration parser). SupersededInboxIDs reads the
// top-level "superseded" array from a triage-decision.json body: deduped,
// order-preserving, and empty (NEVER a panic) on an absent field or invalid JSON.
// This is the data-driven wiring that replaces the prose-only "verify vs HEAD,
// move to consumed" carryover instruction that silently lapsed for 4 cycles.
func TestC548_003_SupersededInboxIDs_ParsesDedupsAndTolerates(t *testing.T) {
	body := []byte(`{"top_n":[{"id":"recover-x"}],"superseded":["loop-self-prioritize-unmet-fleet-concurrency","other-id","loop-self-prioritize-unmet-fleet-concurrency"]}`)
	got := inboxmover.SupersededInboxIDs(body)
	want := []string{"loop-self-prioritize-unmet-fleet-concurrency", "other-id"}
	if len(got) != len(want) {
		t.Fatalf("SupersededInboxIDs = %v, want %v (deduped, order-preserving)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SupersededInboxIDs[%d] = %q, want %q (order must be preserved, dups dropped)", i, got[i], want[i])
		}
	}
	// Absent "superseded" field → empty, not nil-panic.
	if n := len(inboxmover.SupersededInboxIDs([]byte(`{"top_n":[{"id":"x"}]}`))); n != 0 {
		t.Fatalf("absent superseded field returned %d ids, want 0", n)
	}
	// Invalid JSON → empty, never a panic.
	if n := len(inboxmover.SupersededInboxIDs([]byte(`}{ not json`))); n != 0 {
		t.Fatalf("invalid JSON returned %d ids, want 0 (must tolerate, never panic)", n)
	}
}

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
func TestC548_004_ObserverNotRebuilt_NoFleethealthLeaf(t *testing.T) {
	// Compile-time proof the already-shipped observer still lives in package fleet.
	var _ = fleet.StarvationTracker{}
	root := acsassert.RepoRoot(t)
	leaf := filepath.Join(root, "go", "internal", "fleethealth")
	if _, err := os.Stat(leaf); err == nil {
		t.Fatalf("internal/fleethealth exists (%s) — the fleet-starvation observer already ships in internal/fleet; rebuilding it as a leaf resurrects the cycle-542 apicover-graduation regression", leaf)
	}
	src := filepath.Join(root, "go", "internal", "fleet", "starvation.go")
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("internal/fleet/starvation.go absent (%v) — the shipped observer must be REUSED, not rebuilt or relocated (this cycle's fix is the inbox-reconciliation seam, not the observer)", err)
	}
}

// C548_005 — AC-5 (edge / idempotency). Retiring an id that is not present, and
// passing an empty/nil id list, are clean no-ops — no error, nothing retired.
// Guarantees re-run safety: a second ship pass over an already-drained item never
// fails the (best-effort, never-blocks-ship) inbox lifecycle.
func TestC548_005_ReconcileSuperseded_AbsentIDAndEmptyAreCleanNoOp(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	opts := inboxmover.Options{ProjectRoot: root}

	retired, err := inboxmover.ReconcileSuperseded(opts, []string{"never-existed"}, "processed", inboxmover.PromoteOpts{Cycle: "548"})
	if err != nil {
		t.Fatalf("absent id returned error: %v (must be a clean no-op)", err)
	}
	if len(retired) != 0 {
		t.Fatalf("absent id retired %v, want none", retired)
	}

	retired2, err2 := inboxmover.ReconcileSuperseded(opts, nil, "processed", inboxmover.PromoteOpts{Cycle: "548"})
	if err2 != nil {
		t.Fatalf("empty id list returned error: %v (must be a clean no-op)", err2)
	}
	if len(retired2) != 0 {
		t.Fatalf("empty id list retired %v, want none", retired2)
	}
}

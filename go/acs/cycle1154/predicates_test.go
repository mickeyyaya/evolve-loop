//go:build acs

// Package cycle1154 materialises the acceptance criteria for the single task
// triage committed to THIS cycle:
//
//   - router-drop-unknown-phase-entries → `router.ClampPlanToFloorWith`
//     (go/internal/router/floor.go:70) must DROP any PhasePlanEntry whose phase
//     is not in the plan's known-phase set (the same set `ValidatePlan` already
//     computes via `knownPhaseSet`, validate.go:84), recording each removal as a
//     `Clamp` with rule token "drop-unknown-phase" so the drop is never silent.
//
// Every other id in state.json's carryoverTodos was DEFERRED by triage and
// therefore carries ZERO predicates here — R9.3: predicates bind only to
// triage-committed work, and a predicate gating deferred work starves the
// committed task (the cycle-280 failure mode).
//
// Why this task exists: cycles 1151 and 1152 of this exact fleet lane
// (docs-floor-architecture-change-gate) both hard-failed at dispatch with
// "phase gate-wiring-proof: profile not found" — the advisor hallucinated a
// phase name out of policy prose, ValidatePlan flagged it "unknown-phase" but
// is PURE and REPORT-ONLY, and the one plan-mutating step (the floor clamp)
// only ever ADDS phases. The unknown entry therefore survived into dispatch and
// crashed the cycle.
//
// Predicate strategy — all six are BEHAVIORAL over the exported production
// function; none can be greened by adding a magic string:
//
//   - 001 is the crux: a run:true unknown entry must be gone from Entries.
//   - 002 is the non-silence half: exactly one Clamp per drop, carrying the
//     rule token and naming the dropped phase.
//   - 003 is the ANTI-NO-OP negative: an all-known plan must be byte-identical
//     to today's output with zero drop clamps. Without it, "drop everything" or
//     "drop every skipped entry" would green 001/002 while destroying the
//     floor's actual contract.
//   - 004 is the mint-awareness + passthrough edge: a phase minted IN THIS PLAN
//     is KNOWN (ValidatePlan's must-fix rule), and MintPhases itself is carried
//     through untouched — the clamp governs Entries only.
//   - 005 is the edge/OOD axis: run:false unknowns, the empty phase name, and
//     several unknowns at once — plus the PURITY contract (the caller's input
//     plan must be unmutated), which a naive in-place `slices.Delete` breaks.
//   - 006 is the end-to-end regression for the literal incident: a plan
//     carrying "gate-wiring-proof" alongside a real ship chain must come back
//     with the bogus phase gone AND the integrity floor still complete.
package cycle1154

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// dropRule is the clamp rule token the committed task specifies for a removal
// (scout-report.md Task 1 / triage-report.md top_n). Pinning it keeps the drop
// visible in the EXISTING clamp telemetry rather than inventing a second
// channel.
const dropRule = "drop-unknown-phase"

// hallucinated is the exact phase name the advisor emitted in cycles 1151 and
// 1152. It is prose from docs/operations/operating-policy.md:22, never a phase:
// no registry entry, no .evolve/profiles/gate-wiring-proof.json.
const hallucinated = "gate-wiring-proof"

// nonTrivialIn mirrors the router's own test fixture (floor_test.go): the
// default TDD-pin, with a non-trivial cycle so the tdd floor phase applies.
func nonTrivialIn() router.RouteInput {
	return router.RouteInput{
		Cfg: config.RoutingConfig{
			Conditional: map[string]config.CondRule{
				"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"},
			},
		},
		Signals: router.RoutingSignals{
			Scout: router.ScoutSignals{CycleSizeEstimate: "medium", Present: true},
		},
	}
}

func pe(phase string, run bool) router.PhasePlanEntry {
	return router.PhasePlanEntry{Phase: phase, Run: run}
}

// hasEntry reports whether the plan carries an entry for phase at all —
// regardless of Run. A "drop" must remove the ENTRY; flipping Run to false is
// not enough, because dispatch keys off the entry's presence in the plan.
func hasEntry(plan *router.PhasePlan, phase string) bool {
	if plan == nil {
		return false
	}
	for _, e := range plan.Entries {
		if e.Phase == phase {
			return true
		}
	}
	return false
}

func dropClampsFor(clamps []router.Clamp, phase string) []router.Clamp {
	var out []router.Clamp
	for _, c := range clamps {
		if c.Rule != dropRule {
			continue
		}
		// The clamp must identify its phase somewhere structured — the Phase
		// field is the designated home, but Proposed/Forced are accepted since
		// floor.go's existing force() encodes the phase there.
		if c.Phase == phase ||
			c.Proposed == phase+"=run" || c.Proposed == phase+"=skip" ||
			c.Forced == phase+"=drop" {
			out = append(out, c)
		}
	}
	return out
}

// TestC1154_001_clamp_drops_unknown_run_true_entry is the crux predicate: an
// advisor-hallucinated phase scheduled to RUN must not survive the clamp. This
// is the exact shape that reached dispatch in cycles 1151/1152.
//
// Behavioral: calls the production ClampPlanToFloorWith and inspects the
// returned plan. RED today — the clamp only ever adds entries, never removes.
func TestC1154_001_clamp_drops_unknown_run_true_entry(t *testing.T) {
	in := nonTrivialIn()
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		pe("scout", true),
		pe(hallucinated, true),
	}}

	out, _ := router.ClampPlanToFloorWith(in, plan, router.DefaultShipFloor(), false)

	if out == nil {
		t.Fatal("ClampPlanToFloorWith returned a nil plan for a non-nil input")
	}
	if hasEntry(out, hallucinated) {
		t.Errorf("clamped plan still carries the unknown phase %q: %+v — "+
			"it reaches dispatch and fails with 'profile not found', crashing the cycle "+
			"(cycles 1151, 1152)", hallucinated, out.Entries)
	}
	if !hasEntry(out, "scout") {
		t.Errorf("the canonical phase scout was removed too: %+v — the drop must be "+
			"scoped to unknown phases only", out.Entries)
	}
}

// TestC1154_002_drop_is_recorded_as_a_clamp is the non-silence half. floor.go's
// documented pattern is "SKIP is not silent": every disposition the floor makes
// is visible in clamp telemetry. A drop that vanishes an advisor's phase with
// no record makes the next such incident undiagnosable from the run artifacts.
//
// Exactly ONE clamp per dropped phase — a duplicate would double-count in the
// telemetry the router already emits.
func TestC1154_002_drop_is_recorded_as_a_clamp(t *testing.T) {
	in := nonTrivialIn()
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		pe("scout", true),
		pe(hallucinated, true),
	}}

	_, clamps := router.ClampPlanToFloorWith(in, plan, router.DefaultShipFloor(), false)

	got := dropClampsFor(clamps, hallucinated)
	if len(got) != 1 {
		t.Errorf("want exactly 1 %q clamp naming %q, got %d; all clamps=%+v",
			dropRule, hallucinated, len(got), clamps)
	}
}

// TestC1154_003_known_phase_plan_is_untouched is the ANTI-NO-OP negative and
// the strongest anti-gaming signal in this suite. A plan whose every entry is
// canonical must come back with its entries EXACTLY as the pre-change clamp
// produced them and with zero drop clamps.
//
// Without this, "drop every entry", "drop every run:false entry", or "drop
// anything not in the floor" would all green 001 and 002 while gutting the
// advisor's plan — a far worse failure than the one being fixed.
func TestC1154_003_known_phase_plan_is_untouched(t *testing.T) {
	in := nonTrivialIn()
	// Every phase here is canonical (router.go:18) — including skipped ones,
	// which must survive as skip entries, and "retrospective", which is
	// canonical while its "retro" alias is not.
	entries := []router.PhasePlanEntry{
		pe("scout", true), pe("triage", true), pe("tdd", true),
		pe("build", true), pe("audit", true), pe("ship", true),
		pe("memo", false), pe("retrospective", false),
	}
	plan := &router.PhasePlan{Entries: append([]router.PhasePlanEntry(nil), entries...)}

	out, clamps := router.ClampPlanToFloorWith(in, plan, router.DefaultShipFloor(), false)

	if !reflect.DeepEqual(out.Entries, entries) {
		t.Errorf("an all-known plan was modified.\n got: %+v\nwant: %+v", out.Entries, entries)
	}
	for _, c := range clamps {
		if c.Rule == dropRule {
			t.Errorf("drop clamp %+v fired on an all-known plan — the drop must be "+
				"scoped to phases outside the known set", c)
		}
	}
}

// TestC1154_004_minted_and_configured_phases_are_known pins the two ways a
// non-canonical name is nonetheless LEGITIMATE, and the MintPhases passthrough.
//
//   - A phase minted IN THIS PLAN is known (ValidatePlan's mint-aware must-fix,
//     validate.go:24-26 + knownPhaseSet's plan.MintPhases branch). Dropping it
//     would break the advisor's mint capability outright.
//   - A phase configured in the walk (Cfg.Order/Mandatory/Triggers/Conditional)
//     is known even though it is absent from canonicalOrder.
//   - MintPhases itself must be carried through byte-identical: the clamp
//     governs the run/skip Entries, never the set of minted phases.
func TestC1154_004_minted_and_configured_phases_are_known(t *testing.T) {
	const minted = "cycle1154-minted-phase"
	const configured = "cycle1154-configured-phase"

	in := nonTrivialIn()
	in.Cfg.Order = []string{"scout", configured, "ship"}

	mints := []phaseconfig.PhaseConfig{{PhaseSpec: phasespec.PhaseSpec{Name: minted}}}
	plan := &router.PhasePlan{
		Entries: []router.PhasePlanEntry{
			pe("scout", true),
			pe(minted, true),
			pe(configured, false),
			pe(hallucinated, true),
		},
		MintPhases: mints,
	}

	out, _ := router.ClampPlanToFloorWith(in, plan, router.DefaultShipFloor(), false)

	if !hasEntry(out, minted) {
		t.Errorf("the phase minted by THIS plan (%q) was dropped: %+v — a minted phase "+
			"is known (validate.go mint-aware rule); dropping it disables minting", minted, out.Entries)
	}
	if !hasEntry(out, configured) {
		t.Errorf("the configured walk phase %q was dropped: %+v — Cfg.Order membership "+
			"makes a phase known even when it is not in canonicalOrder", configured, out.Entries)
	}
	if hasEntry(out, hallucinated) {
		t.Errorf("the unknown phase %q survived alongside legitimate non-canonical "+
			"phases: %+v", hallucinated, out.Entries)
	}
	if !reflect.DeepEqual(out.MintPhases, mints) {
		t.Errorf("MintPhases changed: got %+v, want %+v — the clamp governs Entries only",
			out.MintPhases, mints)
	}
}

// TestC1154_005_drop_covers_skipped_entries_and_preserves_purity is the
// edge/OOD axis plus the purity contract.
//
//   - run:false unknowns must be dropped too (acceptance criterion: "for both
//     run:true and run:false entries"). A skipped unknown is still a garbage
//     entry that the walk and the telemetry must not see.
//   - The empty phase name is unknown by construction (knownPhaseSet's add()
//     skips ""), so it must go.
//   - Several unknowns in one plan must ALL go, each with its own clamp.
//   - ClampPlanToFloorWith is documented PURE ("returns a NEW plan (input
//     unmutated)"). A drop implemented as an in-place filter over the copied
//     slice header would corrupt the caller's plan — the orchestrator still
//     holds it for the re-plan comparison.
func TestC1154_005_drop_covers_skipped_entries_and_preserves_purity(t *testing.T) {
	in := nonTrivialIn()
	unknowns := []string{hallucinated, "wiring proof", "", "Scout" /* wrong case */}

	entries := []router.PhasePlanEntry{
		pe("scout", true),
		pe(unknowns[0], false), // skipped unknown
		pe(unknowns[1], false), // prose with a space
		pe(unknowns[2], true),  // empty phase name
		pe(unknowns[3], true),  // non-canonical casing
		pe("audit", true),
	}
	before := append([]router.PhasePlanEntry(nil), entries...)
	plan := &router.PhasePlan{Entries: entries}

	out, clamps := router.ClampPlanToFloorWith(in, plan, router.DefaultShipFloor(), false)

	for _, u := range unknowns {
		if hasEntry(out, u) {
			t.Errorf("unknown phase %q survived the clamp: %+v", u, out.Entries)
		}
		if n := len(dropClampsFor(clamps, u)); n != 1 {
			t.Errorf("want exactly 1 %q clamp for %q, got %d; all clamps=%+v",
				dropRule, u, n, clamps)
		}
	}
	if !hasEntry(out, "scout") || !hasEntry(out, "audit") {
		t.Errorf("known phases were collaterally dropped: %+v", out.Entries)
	}
	if !reflect.DeepEqual(plan.Entries, before) {
		t.Errorf("PURITY violated — the caller's input plan was mutated.\n got: %+v\nwant: %+v",
			plan.Entries, before)
	}
}

// TestC1154_006_gate_wiring_proof_regression is the end-to-end regression for
// the literal incident. It reproduces the failing plan shape from cycles 1151
// and 1152 — a real ship-bound chain with the hallucinated phase spliced in —
// and asserts BOTH halves of correctness at once:
//
//  1. the bogus phase is gone (so dispatch never looks for its missing profile);
//  2. the integrity floor is still complete (tdd/build/audit/ship all run) — the
//     drop must not be implemented in a way that also strips the floor's own
//     forced entries.
func TestC1154_006_gate_wiring_proof_regression(t *testing.T) {
	in := nonTrivialIn()
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		pe("scout", true),
		pe("triage", true),
		pe(hallucinated, true), // ← what crashed cycles 1151 and 1152
		pe("build", true),
		pe("ship", true),
	}}

	out, clamps := router.ClampPlanToFloorWith(in, plan, router.DefaultShipFloor(), false)

	if hasEntry(out, hallucinated) {
		t.Errorf("%q survived into the dispatchable plan: %+v", hallucinated, out.Entries)
	}
	if len(dropClampsFor(clamps, hallucinated)) != 1 {
		t.Errorf("the drop of %q was silent or duplicated; clamps=%+v", hallucinated, clamps)
	}
	// The floor must still hold: reaching ship requires tdd+build+audit.
	for _, want := range []string{"tdd", "build", "audit", "ship"} {
		if !planRunsIn(out, want) {
			t.Errorf("integrity floor broken — %q is not running after the clamp: %+v",
				want, out.Entries)
		}
	}
}

// planRunsIn mirrors floor.go's unexported planRuns for this external package:
// an entry with Run==true; an absent phase counts as not running.
func planRunsIn(plan *router.PhasePlan, phase string) bool {
	for _, e := range plan.Entries {
		if e.Phase == phase {
			return e.Run
		}
	}
	return false
}

//go:build acs

package cycle1154

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const dropRule = "drop-unknown-phase"

const hallucinated = "gate-wiring-proof"

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
		if c.Phase == phase ||
			c.Proposed == phase+"=run" || c.Proposed == phase+"=skip" ||
			c.Forced == phase+"=drop" {
			out = append(out, c)
		}
	}
	return out
}

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

func TestC1154_003_known_phase_plan_is_untouched(t *testing.T) {
	in := nonTrivialIn()
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

func TestC1154_005_drop_covers_skipped_entries_and_preserves_purity(t *testing.T) {
	in := nonTrivialIn()
	unknowns := []string{hallucinated, "wiring proof", "", "Scout"}

	entries := []router.PhasePlanEntry{
		pe("scout", true),
		pe(unknowns[0], false),
		pe(unknowns[1], false),
		pe(unknowns[2], true),
		pe(unknowns[3], true),
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

func TestC1154_006_gate_wiring_proof_regression(t *testing.T) {
	in := nonTrivialIn()
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		pe("scout", true),
		pe("triage", true),
		pe(hallucinated, true),
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
	for _, want := range []string{"tdd", "build", "audit", "ship"} {
		if !planRunsIn(out, want) {
			t.Errorf("integrity floor broken — %q is not running after the clamp: %+v",
				want, out.Entries)
		}
	}
}

func planRunsIn(plan *router.PhasePlan, phase string) bool {
	for _, e := range plan.Entries {
		if e.Phase == phase {
			return e.Run
		}
	}
	return false
}

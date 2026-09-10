package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// TestClampPlanToFloorWith_DropsUnavailablePhases — 2026-09-09 token-waste
// root cause #2: an advisor plan that schedules a phase whose persona doc is
// absent is clamped at the floor, so the dispatch/skip/retrospective spend
// never happens. The drop is recorded under its own rule token, distinct from
// the unknown-phase drop, so forensics can tell "not in the catalog" from "in
// the catalog, persona missing".
func TestClampPlanToFloorWith_DropsUnavailablePhases(t *testing.T) {
	in := RouteInput{
		Cfg:               config.RoutingConfig{Order: []string{"scout", "tdd", "build", "amplify-tests", "audit", "ship"}},
		UnavailablePhases: []string{"amplify-tests"},
	}
	plan := &PhasePlan{Entries: []PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "build", Run: true}, {Phase: "amplify-tests", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
	out, clamps := ClampPlanToFloorWith(in, plan, []string{"build", "audit"}, false)
	for _, e := range out.Entries {
		if e.Phase == "amplify-tests" {
			t.Fatalf("unavailable phase survived the clamp: %+v", out.Entries)
		}
	}
	found := false
	for _, c := range clamps {
		if c.Phase == "amplify-tests" && c.Rule == DropUnavailablePhaseRule && c.Forced == "amplify-tests=drop" {
			found = true
		}
	}
	if !found {
		t.Fatalf("drop must be recorded under %q; clamps=%+v", DropUnavailablePhaseRule, clamps)
	}
	// Positive control: with nothing unavailable the same plan keeps the phase.
	in.UnavailablePhases = nil
	out, _ = ClampPlanToFloorWith(in, plan, []string{"build", "audit"}, false)
	if !planRuns(out, "amplify-tests") {
		t.Fatalf("an available optional phase must be kept: %+v", out.Entries)
	}
}

// TestShouldRun_UnavailablePhaseNeverInserted: the legacy trigger path (below
// Advisory, or with no plan) honours the same exclusion — a firing insert_when
// on a persona-less phase records a clamp instead of a dispatch.
func TestShouldRun_UnavailablePhaseNeverInserted(t *testing.T) {
	cfg := config.RoutingConfig{
		Order:         []string{"scout", "build", "amplify-tests", "audit", "ship"},
		MaxInsertions: 3,
		Triggers: map[string]config.RoutingBlock{"amplify-tests": {
			InsertWhen: []config.Condition{{Field: "build.acs_red", Op: "gt", Value: 0}},
		}},
	}
	sig := RoutingSignals{Build: BuildSignals{Present: true, ACSRed: 2}}
	in := RouteInput{Cfg: cfg, Signals: sig}
	if run, _, _ := shouldRun(in, "amplify-tests", 0); !run {
		t.Fatalf("positive control: the trigger must fire for an available phase")
	}
	in.UnavailablePhases = []string{"amplify-tests"}
	run, optional, clamp := shouldRun(in, "amplify-tests", 0)
	if run || !optional || clamp == nil || clamp.Rule != DropUnavailablePhaseRule {
		t.Fatalf("unavailable phase must not run and must record %q; run=%v optional=%v clamp=%+v", DropUnavailablePhaseRule, run, optional, clamp)
	}
}

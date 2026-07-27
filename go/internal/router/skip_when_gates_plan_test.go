package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// skip_when_gates_plan_test.go — cycle-1140, optional-phase-ev-gating-by-cycle-class.
// Before this gate, the Advisory+plan branch of shouldRun returned planRuns()
// directly, so an advisor-inserted optional ran on EVERY cycle class regardless
// of any configured skip_when. The ACS predicates live behind the `acs` build
// tag; these keep the behaviour in the default suite.

func routeC1140(t *testing.T, cycleSize, triggerPhase string, block config.RoutingBlock) RouterDecision {
	t.Helper()
	return Route(RouteInput{
		Current:   "build",
		Verdict:   "PASS",
		Completed: []string{"scout", "build"},
		Signals:   RoutingSignals{Triage: TriageSignals{CycleSize: cycleSize, Present: true}},
		Cfg: config.RoutingConfig{
			Stage:         config.StageAdvisory,
			Mandatory:     []string{"scout", "build", "audit"},
			MaxInsertions: 10,
			Order:         []string{"scout", "build", "coverage-gate", "audit", "ship"},
			Triggers:      map[string]config.RoutingBlock{triggerPhase: block},
		},
		Plan: &PhasePlan{Entries: []PhasePlanEntry{
			{Phase: "scout", Run: true},
			{Phase: "build", Run: true},
			{Phase: "coverage-gate", Run: triggerPhase == "coverage-gate", Justification: "advisor-inserted optional"},
			{Phase: "audit", Run: true},
		}},
	}, nil)
}

func TestShouldRun_SkipWhenGatesAdvisorPlanByCycleClass(t *testing.T) {
	trivialSkip := config.RoutingBlock{
		SkipWhen: []config.Condition{{Field: "cycle_size", Op: "eq", Value: "trivial"}},
	}

	// Condition holds → the optional is declined AND recorded, so the
	// routing-plan artifact cites the skip instead of silently dropping it.
	dec := routeC1140(t, "trivial", "coverage-gate", trivialSkip)
	if dec.NextPhase == "coverage-gate" {
		t.Errorf("NextPhase = %q on a trivial cycle, want the optional gated", dec.NextPhase)
	}
	if !contains(dec.SkipPhases, "coverage-gate") {
		t.Errorf("SkipPhases = %v, want \"coverage-gate\" recorded", dec.SkipPhases)
	}
	var clamped bool
	for _, c := range dec.Clamps {
		if c.Rule == "skip-when-gates-plan" {
			clamped = true
		}
	}
	if !clamped {
		t.Errorf("Clamps = %v, want a skip-when-gates-plan entry (the forensic trail)", dec.Clamps)
	}

	// Condition does NOT hold → the plan still governs. Guards against a "fix"
	// that just disables optionals.
	if dec = routeC1140(t, "medium", "coverage-gate", trivialSkip); dec.NextPhase != "coverage-gate" {
		t.Errorf("NextPhase = %q on a medium cycle, want \"coverage-gate\"", dec.NextPhase)
	}

	// No skip_when configured → the gate is inert even on a trivial cycle: it
	// comes from config, never a Go literal about cycle class.
	if dec = routeC1140(t, "trivial", "coverage-gate", config.RoutingBlock{}); dec.NextPhase != "coverage-gate" {
		t.Errorf("NextPhase = %q with no skip_when, want \"coverage-gate\"", dec.NextPhase)
	}
}

// TestShouldRun_SkipWhenNeverReachesFloorPhase is the integrity-floor negative:
// `ship ⇒ build ∧ audit ∧ (tdd unless trivial)` is non-configurable, so a
// skip_when aimed at a floor phase must never become a floor bypass.
func TestShouldRun_SkipWhenNeverReachesFloorPhase(t *testing.T) {
	dec := routeC1140(t, "trivial", "audit", config.RoutingBlock{
		SkipWhen: []config.Condition{{Field: "cycle_size", Op: "eq", Value: "trivial"}},
	})
	if contains(dec.SkipPhases, "audit") {
		t.Errorf("SkipPhases = %v — the cycle-class gate reached a ship-chain phase", dec.SkipPhases)
	}
	if dec.NextPhase != "audit" {
		t.Errorf("NextPhase = %q, want \"audit\" (floor phase must still run)", dec.NextPhase)
	}
}

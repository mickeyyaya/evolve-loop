package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

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

	if dec = routeC1140(t, "medium", "coverage-gate", trivialSkip); dec.NextPhase != "coverage-gate" {
		t.Errorf("NextPhase = %q on a medium cycle, want \"coverage-gate\"", dec.NextPhase)
	}

	if dec = routeC1140(t, "trivial", "coverage-gate", config.RoutingBlock{}); dec.NextPhase != "coverage-gate" {
		t.Errorf("NextPhase = %q with no skip_when, want \"coverage-gate\"", dec.NextPhase)
	}
}

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

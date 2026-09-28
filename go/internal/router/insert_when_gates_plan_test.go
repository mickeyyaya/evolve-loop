package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

var bugfixRouted = config.RoutingBlock{InsertWhen: []config.Condition{
	{Field: "scout.goal_type", Op: "==", Value: "bugfix"},
	{Field: "audit.failure_class", Op: "eq", Value: "code-audit-fail"},
}}

func routeC1733(t *testing.T, goalType string, enable map[string]config.Enable, triggers map[string]config.RoutingBlock) RouterDecision {
	t.Helper()
	return Route(RouteInput{
		Current:   "triage",
		Verdict:   "PASS",
		Completed: []string{"scout", "triage"},
		Signals:   RoutingSignals{Scout: ScoutSignals{GoalType: goalType, CycleSizeEstimate: "medium", Present: true}},
		Cfg: config.RoutingConfig{
			Stage:         config.StageAdvisory,
			Mandatory:     []string{"scout", "build", "audit"},
			MaxInsertions: 10,
			Order:         []string{"scout", "triage", "fault-localization", "bug-reproduction", "architecture-design", "build", "audit", "ship"},
			Triggers:      triggers,
			PhaseEnable:   enable,
		},
		Plan: &PhasePlan{Entries: []PhasePlanEntry{
			{Phase: "scout", Run: true},
			{Phase: "triage", Run: true},
			{Phase: "fault-localization", Run: true, Justification: "pipeline health needs a root cause"},
			{Phase: "bug-reproduction", Run: true, Justification: "reproduce before fixing"},
			{Phase: "architecture-design", Run: true, Justification: "a cross-cutting refactor"},
			{Phase: "build", Run: true},
			{Phase: "audit", Run: true},
		}},
	}, nil)
}

func trigger1733() map[string]config.RoutingBlock {
	return map[string]config.RoutingBlock{
		"fault-localization": bugfixRouted,
		"bug-reproduction":   bugfixRouted,
	}
}

func clampForces(dec RouterDecision, rule, forced string) bool {
	for _, c := range dec.Clamps {
		if c.Rule == rule && c.Forced == forced {
			return true
		}
	}
	return false
}

func TestShouldRun_APlannedPhaseWhoseTriggerDoesNotFireIsSkipped(t *testing.T) {
	dec := routeC1733(t, "refactor", nil, trigger1733())
	if dec.NextPhase != "architecture-design" {
		t.Fatalf("NextPhase = %q, want architecture-design (ungated, planned)", dec.NextPhase)
	}
	for _, phase := range []string{"fault-localization", "bug-reproduction"} {
		if !contains(dec.SkipPhases, phase) {
			t.Errorf("SkipPhases = %v, want %s skipped on a refactor lane", dec.SkipPhases, phase)
		}
		if !clampForces(dec, "insert-when-gates-plan", phase+"=skip") {
			t.Errorf("Clamps = %+v, want insert-when-gates-plan forcing %s=skip", dec.Clamps, phase)
		}
	}
}

func TestShouldRun_APlannedPhaseWhoseTriggerFiresRuns(t *testing.T) {
	dec := routeC1733(t, "bugfix", nil, trigger1733())
	if dec.NextPhase != "fault-localization" {
		t.Fatalf("NextPhase = %q, want fault-localization on a bugfix lane", dec.NextPhase)
	}
	if clampForces(dec, "insert-when-gates-plan", "fault-localization=skip") {
		t.Errorf("a firing trigger was clamped: %+v", dec.Clamps)
	}
}

func TestShouldRun_ARubricHintKeepsAPlannedPhaseTheAdvisorsJudgment(t *testing.T) {
	largeOnly := []config.Condition{{Field: "scout.cycle_size", Op: "==", Value: "large"}}
	for _, tc := range []struct {
		name  string
		block config.RoutingBlock
		want  string
	}{
		{"hinted", config.RoutingBlock{InsertWhen: largeOnly, RubricHint: []string{"a novel/cross-cutting goal also warrants architecture-design"}}, "architecture-design"},
		{"unhinted", config.RoutingBlock{InsertWhen: largeOnly}, "build"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			triggers := trigger1733()
			triggers["architecture-design"] = tc.block
			if dec := routeC1733(t, "refactor", nil, triggers); dec.NextPhase != tc.want {
				t.Errorf("NextPhase = %q on a medium cycle, want %q", dec.NextPhase, tc.want)
			}
		})
	}
}

func TestShouldRun_AnOperatorEnabledPhaseRunsFromThePlanWithoutItsTrigger(t *testing.T) {
	dec := routeC1733(t, "refactor", map[string]config.Enable{"fault-localization": config.EnableOn}, trigger1733())
	if dec.NextPhase != "fault-localization" {
		t.Errorf("NextPhase = %q, want fault-localization: enabled=on is the operator's decision", dec.NextPhase)
	}
}

func TestShouldRun_ATriggerNeverGatesAPlannedFloorPhase(t *testing.T) {
	dec := Route(RouteInput{
		Current:   "build",
		Verdict:   "PASS",
		Completed: []string{"scout", "build"},
		Signals:   RoutingSignals{Scout: ScoutSignals{GoalType: "refactor", Present: true}},
		Cfg: config.RoutingConfig{
			Stage:         config.StageAdvisory,
			Mandatory:     []string{"scout", "build"},
			MaxInsertions: 10,
			Order:         []string{"scout", "build", "audit", "ship"},
			Triggers:      map[string]config.RoutingBlock{"audit": bugfixRouted},
		},
		Plan: &PhasePlan{Entries: []PhasePlanEntry{{Phase: "scout", Run: true}, {Phase: "build", Run: true}, {Phase: "audit", Run: true}}},
	}, nil)
	if dec.NextPhase != "audit" || contains(dec.SkipPhases, "audit") {
		t.Errorf("NextPhase = %q, SkipPhases = %v: a planned floor phase must run whatever its trigger says", dec.NextPhase, dec.SkipPhases)
	}
}

package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func documentLaneDecision() router.RouterDecision {
	notForADocument := config.RoutingBlock{
		InsertWhen: []config.Condition{{Field: "scout.goal_type", Op: "==", Value: "bugfix"}},
		SkipWhen:   []config.Condition{{Field: config.SignalDeliverableKind, Op: "eq", Value: config.DeliverableKindDocument}},
	}
	return router.Route(router.RouteInput{
		Current:   "triage",
		Verdict:   "PASS",
		Completed: []string{"scout", "triage"},
		Signals: router.RoutingSignals{
			Scout: router.ScoutSignals{Present: true, GoalType: "bugfix", DeliverableKind: config.DeliverableKindDocument},
		},
		Cfg: config.RoutingConfig{
			Stage:         config.StageAdvisory,
			Mandatory:     []string{"scout", "build", "audit"},
			MaxInsertions: 10,
			Order:         []string{"scout", "triage", "fault-localization", "premise-challenge", "architecture-design", "build", "audit", "ship"},
			Triggers: map[string]config.RoutingBlock{
				"fault-localization": notForADocument,
				"premise-challenge":  {InsertWhen: []config.Condition{{Field: "scout.cycle_size", Op: "==", Value: "large"}}},
			},
		},
		Plan: &router.PhasePlan{Entries: []router.PhasePlanEntry{
			{Phase: "scout", Run: true}, {Phase: "triage", Run: true},
			{Phase: "fault-localization", Run: true}, {Phase: "premise-challenge", Run: true},
			{Phase: "architecture-design", Run: true}, {Phase: "build", Run: true}, {Phase: "audit", Run: true},
		}},
	}, nil)
}

func TestRecordRoutingDecision_ARegistryGatedPlannedPhaseEmitsACodedSignal(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center))

	dec := documentLaneDecision()
	o.recordRoutingDecision(context.Background(), 1692, CycleState{WorkspacePath: t.TempDir(), RunID: "run-1692"}, 3, dec)

	want := map[string]string{"fault-localization": router.RuleSkipWhenGatesPlan, "premise-challenge": router.RuleInsertWhenGatesPlan}
	got := map[string]string{}
	for _, e := range events {
		if e.Code != CodePlanPhaseGated {
			continue
		}
		if e.Kind != signalcenter.KindAdvisorWarning || e.Severity != signalcenter.SeverityWarn || e.Module != signalcenter.ModuleOrchestrator || e.Cycle != 1692 || e.RunID != "run-1692" {
			t.Errorf("event = %+v, want a WARN advisor.warning orchestrator event bound to cycle 1692 / run-1692", e)
		}
		if e.Fields["next_phase"] != dec.NextPhase {
			t.Errorf("fields.next_phase = %q, want %q", e.Fields["next_phase"], dec.NextPhase)
		}
		got[e.Phase] = e.Fields["rule"]
	}
	if len(got) != len(want) || got["fault-localization"] != want["fault-localization"] || got["premise-challenge"] != want["premise-challenge"] {
		t.Errorf("gated-phase events = %v, want %v (one per registry-gated planned phase)", got, want)
	}
}

func TestRecordRoutingDecision_AClampThatRemovesNoPlannedPhaseEmitsNoGateSignal(t *testing.T) {
	t.Parallel()
	center := signalcenter.New()
	var events []signalcenter.Event
	center.Subscribe(func(e signalcenter.Event) { events = append(events, e) })
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center))

	dec := router.RouterDecision{NextPhase: "audit", Clamps: []router.Clamp{{Rule: "max-insertions-cap", Proposed: "tester=insert", Forced: "tester=skip"}}}
	o.recordRoutingDecision(context.Background(), 7, CycleState{WorkspacePath: t.TempDir()}, 1, dec)
	for _, e := range events {
		if e.Code == CodePlanPhaseGated {
			t.Errorf("a cap clamp emitted %s: %+v", CodePlanPhaseGated, e)
		}
	}
}

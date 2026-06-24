package routingtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

func TestCoverageHelpersAndAgentPlanner(t *testing.T) {
	sp := scriptedProposer{spec: AgentSpec{
		Proposals: map[string]*router.Proposal{"build": {InsertPhases: []string{"tester"}}},
		ErrorsOn:  map[string]bool{"audit": true},
		Plan: []router.PhasePlanEntry{
			{Phase: "scout", Run: true},
			{Phase: "build", Run: true},
		},
	}}
	if _, err := sp.Propose(router.RouteInput{Current: "audit"}); err == nil {
		t.Fatal("scripted proposer should surface forced proposal errors")
	}
	if got, err := sp.Propose(router.RouteInput{Current: "build"}); err != nil || got == nil || got.InsertPhases[0] != "tester" {
		t.Fatalf("scripted proposal = %+v, err=%v", got, err)
	}
	plan, err := sp.Plan(router.RouteInput{})
	if err != nil || len(plan.Entries) != 2 {
		t.Fatalf("scripted plan = %+v, err=%v", plan, err)
	}
	sp.spec.PlanError = true
	if _, err := sp.Plan(router.RouteInput{}); err == nil {
		t.Fatal("scripted planner should surface forced plan errors")
	}
}

func TestCoverageSetAndDecisionHelpers(t *testing.T) {
	if !sameSet([]string{"b", "a"}, []string{"a", "b"}) || sameSet([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("sameSet should be order-insensitive and length-sensitive")
	}
	if !subset([]string{"a"}, []string{"a", "b"}) || subset([]string{"c"}, []string{"a", "b"}) {
		t.Fatal("subset should require every wanted element")
	}
	ds := []router.RouterDecision{
		{InsertPhases: []string{"tester"}, Clamps: []router.Clamp{{Rule: "floor"}}},
	}
	if !anyDecisionInsert(ds, "tester") || anyDecisionInsert(ds, "missing") {
		t.Fatal("anyDecisionInsert mismatch")
	}
	if !anyDecisionClamp(ds, "floor") || anyDecisionClamp(ds, "missing") {
		t.Fatal("anyDecisionClamp mismatch")
	}
	if !containsPhase([]core.Phase{core.PhaseBuild}, core.PhaseBuild) || containsPhase(nil, core.PhaseAudit) {
		t.Fatal("containsPhase mismatch")
	}
}

func TestCoverageWorkspaceAndRoutingDecisionIO(t *testing.T) {
	root := t.TempDir()
	ws := seedWorkspace(t, root, 3, map[string]string{"scout-report.md": "ok"})
	if _, err := os.Stat(filepath.Join(ws, "scout-report.md")); err != nil {
		t.Fatalf("seeded handoff missing: %v", err)
	}
	decision := router.RouterDecision{NextPhase: "build", InsertPhases: []string{"tester"}}
	raw, _ := json.Marshal(decision)
	if err := os.WriteFile(filepath.Join(ws, "routing-decision-001.json"), raw, 0o644); err != nil {
		t.Fatalf("write decision: %v", err)
	}
	got := readRoutingDecisions(t, ws)
	if len(got) != 1 || got[0].NextPhase != "build" || got[0].InsertPhases[0] != "tester" {
		t.Fatalf("routing decisions = %+v", got)
	}
}

func TestCoverageAdditionalBricksAndPhaseSeq(t *testing.T) {
	s := Scenario("coverage",
		LargeCycle(),
		Backlog(7),
		SeedFailure("code-audit-fail", 2),
		GreenBuild(),
		PhaseEnabled("memo", config.EnableOn),
		IntentRequired(),
		StrictAudit(),
		PhaseVerdict(core.PhaseBuild, "WARN"),
	)
	if s.Signals.CycleSize != "large" || s.Signals.ScoutBacklog != 7 || len(s.FailedAt) != 2 {
		t.Fatalf("brick state mismatch: %+v", s)
	}
	if s.Enable["intent"] != config.EnableOn || !s.Strict {
		t.Fatalf("strict/enable bricks mismatch: enable=%+v strict=%v", s.Enable, s.Strict)
	}
	if s.Enable["memo"] != config.EnableOn || s.Verdicts[string(core.PhaseBuild)] != "WARN" {
		t.Fatalf("enable/verdict bricks mismatch: enable=%+v verdicts=%+v", s.Enable, s.Verdicts)
	}
	assertPhaseSeq(t, []core.Phase{core.PhaseScout, core.PhaseBuild}, []core.Phase{core.PhaseScout, core.PhaseBuild})
}

func TestCoverageInvariantsAndExpectations(t *testing.T) {
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "ship", Run: true},
		{Phase: "build", Run: true},
		{Phase: "audit", Run: true},
		{Phase: "audit", Run: false},
	}}
	if dups := duplicatePlanPhases(plan); len(dups) != 1 || dups[0] != "audit" {
		t.Fatalf("duplicatePlanPhases=%v", dups)
	}
	if !planRunsTest(plan, "ship") || planRunsTest(plan, "missing") {
		t.Fatal("planRunsTest mismatch")
	}
	assertInvariants(t, ScenarioSpec{Expect: ExpectSpec{Invariants: []string{
		"proposal-never-weakens",
		"mandatory-never-skipped",
		"no-ship-before-audit",
		"tdd-pin-nontrivial",
		"insert-le-cap",
		"determinism",
		"ship-implies-audit-in-plan",
	}}}, router.RouteInput{
		Current:   "build",
		Verdict:   "PASS",
		Cfg:       buildConfig(ScenarioSpec{MaxInsertions: 2}),
		Completed: []string{"scout", "build", "audit"},
		Plan:      &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "ship", Run: true}, {Phase: "build", Run: true}, {Phase: "audit", Run: true}}},
	}, &router.Proposal{InsertPhases: []string{"tester"}}, router.Route(router.RouteInput{
		Current:   "build",
		Verdict:   "PASS",
		Cfg:       buildConfig(ScenarioSpec{MaxInsertions: 2}),
		Completed: []string{"scout", "build", "audit"},
	}, &router.Proposal{InsertPhases: []string{"tester"}}))
}

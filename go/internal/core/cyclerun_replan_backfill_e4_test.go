package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type replanFailingPlannerE4 struct{ calls int }

func (p *replanFailingPlannerE4) Plan(router.RouteInput) (*router.PhasePlan, error) {
	return nil, nil
}
func (p *replanFailingPlannerE4) RePlan(router.RouteInput) (*router.PhasePlan, error) {
	p.calls++
	return nil, errors.New("advisor unreachable")
}

func replanRunE4(t *testing.T, o *Orchestrator, ws string, initial *router.PhasePlan) *cycleRun {
	t.Helper()
	return &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: ws}, cycle: 9,
		cs:          CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout"}},
		envSnap:     map[string]string{},
		clampedPlan: initial,
	}
}

func replanScoutOnlyPlanE4() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{{Phase: "scout", Run: true}}}
}

func replanFullPlanE4() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "build", Run: true}, {Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
}

func TestPostScoutReplan_OnlyTheAdvisorModeReplans(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		stage config.Stage
		mode  config.Mode
	}{
		{"routing below advisory", config.StageShadow, config.ModeDynamicLLM},
		{"routing off", config.StageOff, config.ModeDynamicLLM},
		{"a static preset at advisory", config.StageAdvisory, config.ModeStaticPreset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := scoutWorkspace(t)
			pl := &replanPlanner{plan: replanFullPlanE4()}
			o := replanOrchestrator(t, &fakeLedger{}, pl)
			o.cfg.Stage, o.cfg.Mode = tc.stage, tc.mode
			initial := replanScoutOnlyPlanE4()
			cr := replanRunE4(t, o, ws, initial)

			cr.postScoutReplan()

			if pl.replanCalls != 0 || cr.replanDepth != 0 || cr.clampedPlan != initial {
				t.Fatalf("calls=%d depth=%d: only DynamicLLM at advisory or above may re-plan", pl.replanCalls, cr.replanDepth)
			}
			if _, err := os.Stat(filepath.Join(ws, "phase-replan.json")); err == nil {
				t.Fatal("a gated-out re-plan must record nothing")
			}
		})
	}
}

func TestPostScoutReplan_AnUnreadableWorkspaceWarnsAndNeverReplans(t *testing.T) {
	t.Parallel()

	pl := &replanPlanner{plan: replanFullPlanE4()}
	cr := replanRunE4(t, replanOrchestrator(t, &fakeLedger{}, pl), filepath.Join(t.TempDir(), "gone"), replanScoutOnlyPlanE4())

	var sink injectedStderr
	cr.o.stderr = &sink
	cr.postScoutReplan()
	log := sink.String()

	if !strings.Contains(log, "WARN post-scout digest degraded (cycle 9): router digest: workspace:") {
		t.Fatalf("the degraded digest must be named:\n%s", log)
	}
	if pl.replanCalls != 0 {
		t.Fatalf("with no measured scout signals nothing may be re-planned; calls=%d", pl.replanCalls)
	}
}

func TestPostScoutReplan_APlanThatAlreadyCoversTheMeasuredNeedIsLeftAlone(t *testing.T) {
	t.Parallel()

	ws := scoutWorkspace(t)
	pl := &replanPlanner{plan: replanFullPlanE4()}
	covering := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "tester", Run: true}, {Phase: "build", Run: true}, {Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
	cr := replanRunE4(t, replanOrchestrator(t, &fakeLedger{}, pl), ws, covering)

	cr.postScoutReplan()

	if pl.replanCalls != 0 || cr.replanDepth != 0 || cr.clampedPlan != covering {
		t.Fatalf("calls=%d depth=%d: a plan with no material divergence must not be re-planned", pl.replanCalls, cr.replanDepth)
	}
}

func TestPostScoutReplan_AZeroDepthCapMeansOneReplan(t *testing.T) {
	t.Parallel()

	ws := scoutWorkspace(t)
	led := &fakeLedger{}
	pl := &replanPlanner{plan: replanFullPlanE4()}
	o := replanOrchestrator(t, led, pl)
	o.cfg.RePlanMaxDepth = 0
	cr := replanRunE4(t, o, ws, replanScoutOnlyPlanE4())

	var sink injectedStderr
	cr.o.stderr = &sink
	cr.postScoutReplan()
	cr.postScoutReplan()
	log := sink.String()

	if pl.replanCalls != 1 || cr.replanDepth != 1 {
		t.Fatalf("calls=%d depth=%d, want exactly one re-plan under the default cap of 1", pl.replanCalls, cr.replanDepth)
	}
	if !strings.Contains(log, "re-plan depth cap (1) reached at cycle 9") {
		t.Fatalf("the default cap must be the one reported:\n%s", log)
	}
}

func TestPostScoutReplan_AFailingAdvisorWarnsKeepsThePlanAndSpendsNoDepth(t *testing.T) {
	t.Parallel()

	ws := scoutWorkspace(t)
	pl := &replanFailingPlannerE4{}
	o := replanOrchestrator(t, &fakeLedger{}, &replanPlanner{})
	o.planner = pl
	initial := replanScoutOnlyPlanE4()
	cr := replanRunE4(t, o, ws, initial)

	var sink injectedStderr
	cr.o.stderr = &sink
	cr.postScoutReplan()
	log := sink.String()

	if pl.calls != 1 || cr.replanDepth != 0 || cr.clampedPlan != initial {
		t.Fatalf("calls=%d depth=%d: a failed re-plan must fail open", pl.calls, cr.replanDepth)
	}
	if !strings.Contains(log, "WARN post-scout re-plan failed (keeping initial plan): advisor unreachable") {
		t.Fatalf("the failure must be loud:\n%s", log)
	}
	if _, err := os.Stat(filepath.Join(ws, "phase-replan.json")); err == nil {
		t.Fatal("a failed re-plan must not write phase-replan.json")
	}
}

func TestRecordReplanEscalation_ALostLedgerRowWarnsAndNeverPanics(t *testing.T) {
	t.Parallel()

	ws := scoutWorkspace(t)
	led := &fakeLedger{failOnAppend: true}
	pl := &replanPlanner{plan: replanFullPlanE4()}
	cr := replanRunE4(t, replanOrchestrator(t, led, pl), ws, replanScoutOnlyPlanE4())
	cr.replanDepth = 1

	var sink injectedStderr
	cr.o.stderr = &sink
	cr.postScoutReplan()
	log := sink.String()

	if pl.replanCalls != 0 {
		t.Fatalf("at the cap nothing may be re-planned; calls=%d", pl.replanCalls)
	}
	if !strings.Contains(log, "WARN replan_escalation ledger append: forced ledger append fail") {
		t.Fatalf("a lost escalation row must warn:\n%s", log)
	}
}

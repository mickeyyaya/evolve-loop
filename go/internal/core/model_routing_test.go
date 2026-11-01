package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func modelRoutingCfg(mr config.ModelRouting) config.RoutingConfig {
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.ModelRouting = mr
	return cfg
}

type modelRoutingPlanner struct {
	plan *router.PhasePlan
	err  error
}

func (p *modelRoutingPlanner) Plan(router.RouteInput) (*router.PhasePlan, error) {
	return p.plan, p.err
}

func buildProposingPlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true},
		{Phase: "build", Run: true, Tier: "deep"},
		{Phase: "audit", Run: true},
		{Phase: "ship", Run: true},
	}}
}

func TestModelRouting_AutoApplies(t *testing.T) {
	t.Parallel()

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners,
		WithRouting(modelRoutingCfg(config.ModelRoutingAuto), router.StaticPreset{}),
		WithPlanner(&modelRoutingPlanner{plan: buildProposingPlan()}))

	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true,
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	fr := runners[PhaseBuild].(*fakeRunner)
	if len(fr.requests) == 0 {
		t.Fatal("build phase never dispatched")
	}
	req := fr.requests[0]
	if !req.Driver.IsZero() || req.Assignment.Tier != "deep" {
		t.Errorf("Driver/Tier = %q/%q, want no driver and the tier deep (auto applies the clamped tier; the routing data picks the driver)", req.Driver, req.Assignment.Tier)
	}
}

func TestModelRouting_AdvisoryLogsNotApplies(t *testing.T) {
	t.Parallel()

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners,
		WithRouting(modelRoutingCfg(config.ModelRoutingAdvisory), router.StaticPreset{}),
		WithPlanner(&modelRoutingPlanner{plan: buildProposingPlan()}))

	projectRoot := t.TempDir()
	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: projectRoot, GoalHash: "g", DisableWorkspaceGuard: true,
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	fr := runners[PhaseBuild].(*fakeRunner)
	if len(fr.requests) == 0 {
		t.Fatal("build phase never dispatched")
	}
	if req := fr.requests[0]; !req.Driver.IsZero() || req.Assignment.Tier != "" {
		t.Errorf("Driver/Tier = %q/%q, want empty (advisory must NOT apply to dispatch)", req.Driver, req.Assignment.Tier)
	}

	ws := RunWorkspacePath(projectRoot, res.Cycle)
	raw, rerr := os.ReadFile(filepath.Join(ws, "phase-plan.json"))
	if rerr != nil {
		t.Fatalf("read phase-plan.json: %v", rerr)
	}
	var entries []router.PhasePlanEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("unmarshal phase-plan.json: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Phase == "build" {
			found = true
			if e.Tier != "deep" {
				t.Errorf("recorded build entry = %+v, want the clamped tier deep logged", e)
			}
		}
	}
	if !found {
		t.Fatal("phase-plan.json has no build entry")
	}
}

func TestModelRouting_StaticIsNoop(t *testing.T) {
	t.Parallel()

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners,
		WithRouting(modelRoutingCfg(config.ModelRoutingStatic), router.StaticPreset{}),
		WithPlanner(&modelRoutingPlanner{plan: buildProposingPlan()}))

	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true,
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	fr := runners[PhaseBuild].(*fakeRunner)
	if len(fr.requests) == 0 {
		t.Fatal("build phase never dispatched")
	}
	if req := fr.requests[0]; !req.Driver.IsZero() || req.Assignment.Tier != "" {
		t.Errorf("Driver/Tier = %q/%q, want empty (static is a noop)", req.Driver, req.Assignment.Tier)
	}
}

func TestModelRouting_AutoDegradesToProfileStatic(t *testing.T) {
	t.Parallel()

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners,
		WithRouting(modelRoutingCfg(config.ModelRoutingAuto), router.StaticPreset{}),
		WithPlanner(&modelRoutingPlanner{err: errors.New("advisor outage: exit=81")}))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true,
	})
	if err != nil {
		t.Fatalf("RunCycle must degrade gracefully, not error: %v", err)
	}
	if indexOfPhase(res.PhasesRun, "build") < 0 {
		t.Fatalf("build never ran — an advisor outage under auto must degrade to the static spine, not break dispatch (PhasesRun=%v)", res.PhasesRun)
	}
	fr := runners[PhaseBuild].(*fakeRunner)
	if len(fr.requests) == 0 {
		t.Fatal("build phase never dispatched")
	}
	if req := fr.requests[0]; !req.Driver.IsZero() || req.Assignment.Tier != "" {
		t.Errorf("Driver/Tier = %q/%q, want empty (nil plan ⇒ no overlay, ever)", req.Driver, req.Assignment.Tier)
	}
}

func TestPhaseRequest_CarriesANeutralAssignmentAndNoDriverOnTheWire(t *testing.T) {
	t.Parallel()

	req := PhaseRequest{Cycle: 1, Assignment: TaskAssignment{Tier: "deep", Effort: "high"}, Driver: llmroute.Bind("codex-tmux")}
	buf, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(buf), "codex") || strings.Contains(string(buf), "cli") || !strings.Contains(string(buf), `"assignment":{"tier":"deep","effort":"high"}`) {
		t.Errorf("the request JSON carries the neutral assignment and never a driver: %s", buf)
	}
	var back PhaseRequest
	if err := json.Unmarshal(buf, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Assignment != req.Assignment || !back.Driver.IsZero() {
		t.Errorf("round trip = %+v / %q, want the assignment and no driver", back.Assignment, back.Driver)
	}
}

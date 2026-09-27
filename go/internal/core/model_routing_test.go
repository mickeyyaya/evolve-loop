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
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// modelRoutingCfg builds a DynamicLLM cfg at StageAdvisory, the combination
// that opens the planner gate (Stage>=Advisory && Mode==DynamicLLM &&
// planner!=nil) required for the whole-cycle plan to run.
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
		{Phase: "build", Run: true, CLI: "claude-tmux", Tier: "deep"},
		{Phase: "audit", Run: true},
		{Phase: "ship", Run: true},
	}}
}

func TestModelRouting_AutoApplies(t *testing.T) {
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
	if req.ModelRoutingCLI != "claude-tmux" || req.ModelRoutingTier != "deep" {
		t.Errorf("ModelRoutingCLI/Tier = %q/%q, want claude-tmux/deep (auto applies the clamped plan proposal)", req.ModelRoutingCLI, req.ModelRoutingTier)
	}
}

func TestModelRouting_AdvisoryLogsNotApplies(t *testing.T) {
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
	if req := fr.requests[0]; req.ModelRoutingCLI != "" || req.ModelRoutingTier != "" {
		t.Errorf("ModelRoutingCLI/Tier = %q/%q, want empty (advisory must NOT apply to dispatch)", req.ModelRoutingCLI, req.ModelRoutingTier)
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
			if e.CLI != "claude-tmux" || e.Tier != "deep" {
				t.Errorf("recorded build entry = %+v, want the clamped {claude-tmux,deep} proposal logged", e)
			}
		}
	}
	if !found {
		t.Fatal("phase-plan.json has no build entry")
	}
}

func TestModelRouting_StaticIsNoop(t *testing.T) {
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
	if req := fr.requests[0]; req.ModelRoutingCLI != "" || req.ModelRoutingTier != "" {
		t.Errorf("ModelRoutingCLI/Tier = %q/%q, want empty (static is a noop)", req.ModelRoutingCLI, req.ModelRoutingTier)
	}
}

func TestModelRouting_AutoDegradesToProfileStatic(t *testing.T) {
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
	if req := fr.requests[0]; req.ModelRoutingCLI != "" || req.ModelRoutingTier != "" {
		t.Errorf("ModelRoutingCLI/Tier = %q/%q, want empty (nil plan ⇒ no overlay, ever)", req.ModelRoutingCLI, req.ModelRoutingTier)
	}
}

// Proves the injected catalog lookup actually reaches the clamp, not merely
// stored and ignored.
func TestModelRouting_CatalogMissClampsUnderAuto(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	alwaysMiss := func(cli, tier string) (string, bool) { return "", false }
	o := NewOrchestrator(st, &fakeLedger{}, runners,
		WithRouting(modelRoutingCfg(config.ModelRoutingAuto), router.StaticPreset{}),
		WithPlanner(&modelRoutingPlanner{plan: buildProposingPlan()}),
		WithModelCatalogLookup(alwaysMiss))

	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true,
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	fr := runners[PhaseBuild].(*fakeRunner)
	if len(fr.requests) == 0 {
		t.Fatal("build phase never dispatched")
	}
	if req := fr.requests[0]; req.ModelRoutingCLI != "" || req.ModelRoutingTier != "" {
		t.Errorf("ModelRoutingCLI/Tier = %q/%q, want empty — the injected catalog lookup reports every pair as unresolvable, so the clamp must reject it even under auto", req.ModelRoutingCLI, req.ModelRoutingTier)
	}
}

func TestPhaseRequest_ModelRoutingFieldsOmitEmptyByDefault(t *testing.T) {
	buf, err := json.Marshal(PhaseRequest{Cycle: 1, ProjectRoot: "/p", GoalHash: "g"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(buf), "model_routing_cli") || strings.Contains(string(buf), "model_routing_tier") {
		t.Errorf("zero-value PhaseRequest JSON contains model_routing_* keys: %s", buf)
	}
}

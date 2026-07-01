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

// modelRoutingCfg builds a DynamicLLM cfg at StageAdvisory with the given
// model-routing axis — the cycle-440 MR4 wiring gate (o.cfg.Stage >=
// StageAdvisory && o.cfg.Mode == ModeDynamicLLM && o.planner != nil) is what
// makes the whole-cycle plan (and, once wired, ClampPlanModelRouting) run at
// all.
func modelRoutingCfg(mr config.ModelRouting) config.RoutingConfig {
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.ModelRouting = mr
	return cfg
}

// modelRoutingPlanner returns a fixed plan proposing {claude-tmux, deep} for
// the build phase (the phase this suite inspects requests for).
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

// TestModelRouting_AutoApplies (mr4-projection AC1): under model_routing=auto,
// the build phase's PhaseRequest carries the plan's (clamped) {cli,tier}
// proposal — the only mode that can change actual dispatch (I2).
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

// TestModelRouting_AdvisoryLogsNotApplies (mr4-projection AC2): under
// model_routing=advisory, the SAME plan proposal is computed and RECORDED
// (phase-plan.json carries the clamped {cli,tier}) but the PhaseRequest
// fields dispatched to the build phase stay empty — advisory logs, it never
// applies.
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

	// "Logs" half of the contract: the clamped proposal must still have been
	// computed and persisted to phase-plan.json (proves ClampPlanModelRouting
	// ran under advisory too — I2 — it just isn't projected onto the request).
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

// TestModelRouting_StaticIsNoop (mr4-projection AC3, I8 byte-identical
// regression floor): with model_routing left at its Go zero value (static),
// the plan's {cli,tier} proposal never reaches the PhaseRequest — dispatch is
// byte-identical to pre-MR4 behavior even though the advisor proposed
// something.
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

// TestModelRouting_AutoDegradesToProfileStatic (mr4-projection AC4, I4 HARD
// CONSTRAINT): under model_routing=auto, a failed/absent advisor (Plan
// returns an error, so clampedPlan stays nil — the documented exit=81
// outage mode) must NEVER break dispatch. Every phase still runs with empty
// overlay fields — profile-static per phase — and the cycle completes.
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

// TestModelRouting_CatalogMissClampsUnderAuto (mr4-projection AC1
// counterpart / DI wiring): WithModelCatalogLookup injects the catalog-
// resolvability gate into router.ClampPlanModelRouting. When the injected
// lookup reports a miss for every {cli,tier}, an in-bounds-guardrail proposal
// must still be clamped away — even under auto — proving the DI seam
// actually reaches the clamp (not merely stored and ignored).
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

// TestPhaseRequest_ModelRoutingFieldsOmitEmptyByDefault (I8): the two new
// wire fields are omitempty — a zero-value PhaseRequest (the entire
// pre-cycle-440 fleet) marshals with neither key present, so an unaware
// consumer (e.g. the subprocess phaseproto override path) sees no new shape.
func TestPhaseRequest_ModelRoutingFieldsOmitEmptyByDefault(t *testing.T) {
	buf, err := json.Marshal(PhaseRequest{Cycle: 1, ProjectRoot: "/p", GoalHash: "g"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(buf), "model_routing_cli") || strings.Contains(string(buf), "model_routing_tier") {
		t.Errorf("zero-value PhaseRequest JSON contains model_routing_* keys: %s", buf)
	}
}

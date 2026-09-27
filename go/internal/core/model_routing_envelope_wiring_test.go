package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// envelopeProbePlan proposes a single {cli,tier} for the build phase (the phase
// whose fakeRunner request this suite inspects). CLI is left as a real driver so
// the entry is a genuine advisor proposal; only the tier is under test.
func envelopeProbePlan(tier string) *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true},
		{Phase: "build", Run: true, CLI: "claude-tmux", Tier: tier},
		{Phase: "audit", Run: true},
		{Phase: "ship", Run: true},
	}}
}

// writeBuilderProfile writes .evolve/profiles/builder.json under root (the
// AGENT-named file the "build" phase resolves to: strip "evolve-" from
// AgentPromptName "evolve-builder"). envelope may be "" for the no-envelope
// (universal-floor) case. The file is intentionally minimal — no $include_policy
// sentinels — so profiles.Loader.Get parses it without a tool-policy file.
func writeBuilderProfile(t *testing.T, root, envelope string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	body := `{"name":"builder","role":"builder","model_tier_default":"balanced"`
	if envelope != "" {
		body += `,"model_tier_envelope":` + envelope
	}
	body += "}\n"
	if err := os.WriteFile(filepath.Join(dir, "builder.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write builder.json: %v", err)
	}
}

// runBuildTierThroughCycle drives a full RunCycle under model_routing=auto with
// the given proposed build tier and returns the tier the build phase was
// actually dispatched with (empty when the guard emptied the whole proposal).
// projectRoot must already contain any .evolve/profiles/*.json fixtures.
func runBuildTierThroughCycle(t *testing.T, projectRoot, proposedTier string) string {
	t.Helper()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners,
		WithRouting(modelRoutingCfg(config.ModelRoutingAuto), router.StaticPreset{}),
		WithPlanner(&modelRoutingPlanner{plan: envelopeProbePlan(proposedTier)}))

	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: projectRoot, GoalHash: "g", DisableWorkspaceGuard: true,
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	fr := runners[PhaseBuild].(*fakeRunner)
	if len(fr.requests) == 0 {
		t.Fatal("build phase never dispatched")
	}
	return fr.requests[0].ModelRoutingTier
}

func TestModelTierEnvelope_CeilingClampsThroughRealProfileLookup(t *testing.T) {
	root := t.TempDir()
	writeBuilderProfile(t, root, `{"min":"balanced","max":"deep"}`)

	got := runBuildTierThroughCycle(t, root, "top")
	if got != "deep" {
		t.Errorf("dispatched build tier = %q, want \"deep\" — an above-ceiling advisor tier must clamp DOWN to the phase profile's explicit envelope max through the REAL profileForModelRouting seam (nil-stub leaves it %q unclamped)", got, "top")
	}
}

func TestModelTierEnvelope_UniversalFloorClampsThroughRealDispatch(t *testing.T) {
	root := t.TempDir()
	writeBuilderProfile(t, root, "") // no model_tier_envelope → universal floor governs

	got := runBuildTierThroughCycle(t, root, "fast")
	if got != "balanced" {
		t.Errorf("dispatched build tier = %q, want \"balanced\" — an envelope-less profile must still clamp a below-floor tier UP to universalTierFloor.Min through the real dispatch path (nil-stub leaves it %q unclamped)", got, "fast")
	}
}

// Unlike its siblings, this holds on both the nil-stub and the wired path —
// it pins precision (no over-clamp), not the wiring fix itself.
func TestModelTierEnvelope_WithinEnvelopeTierPassesThrough(t *testing.T) {
	root := t.TempDir()
	writeBuilderProfile(t, root, `{"min":"balanced","max":"deep"}`)

	got := runBuildTierThroughCycle(t, root, "deep")
	if got != "deep" {
		t.Errorf("dispatched build tier = %q, want \"deep\" — a within-envelope tier must pass through unclamped (guard must not over-clamp legal proposals)", got)
	}
}

func TestModelTierEnvelope_AbsentProfileDegradesNilSafe(t *testing.T) {
	root := t.TempDir() // no .evolve/profiles/ at all

	got := runBuildTierThroughCycle(t, root, "fast")
	if got != "fast" {
		t.Errorf("dispatched build tier = %q, want \"fast\" — a phase with no profile on disk must degrade nil-safe (no clamp, no error), matching ValidatePin's nil-profile pass-through contract", got)
	}
}

func TestModelTierEnvelope_ClampRecordedInPhasePlan(t *testing.T) {
	root := t.TempDir()
	writeBuilderProfile(t, root, `{"min":"balanced","max":"deep"}`)

	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners,
		WithRouting(modelRoutingCfg(config.ModelRoutingAuto), router.StaticPreset{}),
		WithPlanner(&modelRoutingPlanner{plan: envelopeProbePlan("top")}))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	ws := RunWorkspacePath(root, res.Cycle)
	raw, rerr := os.ReadFile(filepath.Join(ws, "phase-plan.json"))
	if rerr != nil {
		t.Fatalf("read phase-plan.json: %v", rerr)
	}
	var entries []router.PhasePlanEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("unmarshal phase-plan.json: %v", err)
	}
	for _, e := range entries {
		if e.Phase == "build" {
			if e.Tier != "deep" {
				t.Errorf("recorded build entry tier = %q, want \"deep\" — the ceiling clamp must be reflected in phase-plan.json, proving the guard fired in the composed path", e.Tier)
			}
			return
		}
	}
	t.Fatal("phase-plan.json has no build entry")
}

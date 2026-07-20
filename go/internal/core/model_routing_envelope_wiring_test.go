package core

// Cycle-976 RED wiring proofs for the model-tier-envelope guard.
//
// These are INTEGRATION tests through RunCycle (not the router unit boundary):
// they drive the REAL Orchestrator.profileForModelRouting seam so they fail
// against the current permanent nil-stub (cyclerun.go:711-713) and pass once
// Builder wires a real per-phase profile lookup. The router-level guard
// (router.ClampPlanModelRouting) is already unit-covered and correct; the defect
// under test is purely that the production DI seam feeding it real profiles
// returns nil for every phase, silently disabling floor AND ceiling clamps
// (including the documented "universal floor") for 100% of live cycles.
//
// Why RED now: profileForModelRouting returns nil → prof==nil in the guard →
// the whole envelope branch (model_routing_clamp.go:88-109) is skipped → the
// advisor-proposed out-of-envelope tier reaches dispatch verbatim. The
// assertions below want the CLAMPED tier, so they fail on the assertion (right
// reason), not on a compile error — every helper used here already exists in
// the core test package.

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

// TestModelTierEnvelope_CeilingClampsThroughRealProfileLookup — Task-1 wiring
// proof (ceiling / explicit envelope). A phase whose OWN profile declares
// model_tier_envelope {min:balanced,max:deep} must clamp an advisor proposal of
// "top" (rank 4 > deep rank 3) DOWN to "deep" when driven through the real
// Orchestrator seam. RED on the nil-stub (dispatched tier stays "top"); GREEN
// once profileForModelRouting resolves builder.json.
func TestModelTierEnvelope_CeilingClampsThroughRealProfileLookup(t *testing.T) {
	root := t.TempDir()
	writeBuilderProfile(t, root, `{"min":"balanced","max":"deep"}`)

	got := runBuildTierThroughCycle(t, root, "top")
	if got != "deep" {
		t.Errorf("dispatched build tier = %q, want \"deep\" — an above-ceiling advisor tier must clamp DOWN to the phase profile's explicit envelope max through the REAL profileForModelRouting seam (nil-stub leaves it %q unclamped)", got, "top")
	}
}

// TestModelTierEnvelope_UniversalFloorClampsThroughRealDispatch — Task-2 wiring
// proof (universal floor). A phase whose profile declares NO envelope must still
// clamp a below-floor proposal ("fast", rank 1) UP to the compiled
// universalTierFloor.Min ("balanced", rank 2) in the composed production path,
// not just at the router unit boundary. RED on the nil-stub (prof==nil skips the
// universalTierFloor substitution entirely, so "fast" reaches dispatch); GREEN
// once the real profile (non-nil, envelope-less) is resolved.
func TestModelTierEnvelope_UniversalFloorClampsThroughRealDispatch(t *testing.T) {
	root := t.TempDir()
	writeBuilderProfile(t, root, "") // no model_tier_envelope → universal floor governs

	got := runBuildTierThroughCycle(t, root, "fast")
	if got != "balanced" {
		t.Errorf("dispatched build tier = %q, want \"balanced\" — an envelope-less profile must still clamp a below-floor tier UP to universalTierFloor.Min through the real dispatch path (nil-stub leaves it %q unclamped)", got, "fast")
	}
}

// TestModelTierEnvelope_WithinEnvelopeTierPassesThrough — anti-no-op / precision
// guard (semantic axis). A proposal INSIDE the phase's envelope
// ({min:balanced,max:deep}, proposing "deep") must reach dispatch UNCHANGED: the
// wiring must clamp only genuine violations, never degenerate into "always clamp
// to the floor/default". Holds on both the nil-stub and the wired path, so it
// pins the fix's precision rather than discriminating the wiring — its value is
// catching a Builder over-clamp regression.
func TestModelTierEnvelope_WithinEnvelopeTierPassesThrough(t *testing.T) {
	root := t.TempDir()
	writeBuilderProfile(t, root, `{"min":"balanced","max":"deep"}`)

	got := runBuildTierThroughCycle(t, root, "deep")
	if got != "deep" {
		t.Errorf("dispatched build tier = %q, want \"deep\" — a within-envelope tier must pass through unclamped (guard must not over-clamp legal proposals)", got)
	}
}

// TestModelTierEnvelope_AbsentProfileDegradesNilSafe — nil-safety invariant
// (edge axis, Task-1 AC "nil only for phases genuinely lacking one"). When NO
// profile file exists for the phase, the real lookup must resolve to nil and the
// guard must degrade to the documented pass-through (ValidatePin's nil-profile
// contract) rather than erroring or emptying the proposal. A below-floor "fast"
// proposal survives to dispatch because there is no profile — and, critically,
// no universal floor is applied when the profile is genuinely ABSENT (distinct
// from present-but-envelope-less, which DOES get the universal floor above).
func TestModelTierEnvelope_AbsentProfileDegradesNilSafe(t *testing.T) {
	root := t.TempDir() // no .evolve/profiles/ at all

	got := runBuildTierThroughCycle(t, root, "fast")
	if got != "fast" {
		t.Errorf("dispatched build tier = %q, want \"fast\" — a phase with no profile on disk must degrade nil-safe (no clamp, no error), matching ValidatePin's nil-profile pass-through contract", got)
	}
}

// TestModelTierEnvelope_ClampRecordedInPhasePlan — evidence axis: the ceiling
// clamp must also be persisted to phase-plan.json (the operator-visible
// artifact), not silently applied — mirroring the existing advisory/auto logging
// contract. RED on the nil-stub (no clamp recorded because none fires).
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

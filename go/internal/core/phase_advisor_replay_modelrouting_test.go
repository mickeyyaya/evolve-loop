package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// A recorded advisor plan response carrying a per-phase {cli,tier} proposal
// drives the same chain a live model_routing=auto cycle runs end to end:
// floor clamp, model-routing guardrail clamp, then the soft dispatch overlay.
func TestReplayPlanFromResponse_ModelRoutingOverlayAppliesUnderAuto(t *testing.T) {
	t.Parallel()
	raw := `[{"phase":"build","run":true,"justification":"needs deep reasoning","cli":"codex","tier":"balanced"}]`
	in := router.RouteInput{}
	floor := []string{} // no ship-chain in this fixture; isolate the model-routing clamp

	floored, _, err := ReplayPlanFromResponse(raw, in, floor)
	if err != nil {
		t.Fatalf("ReplayPlanFromResponse: %v", err)
	}

	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude", "codex"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"codex": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"balanced": "gpt-5-codex"}},
	}}
	mrPlan, mrClamps := router.ClampPlanModelRouting(floored, func(string) *profiles.Profile { return prof }, catalog.Lookup)
	if len(mrClamps) != 0 {
		t.Fatalf("in-bounds proposal must not clamp; clamps=%+v", mrClamps)
	}

	var entry *router.PhasePlanEntry
	for i := range mrPlan.Entries {
		if mrPlan.Entries[i].Phase == "build" {
			entry = &mrPlan.Entries[i]
		}
	}
	if entry == nil {
		t.Fatal("no build entry in the clamped plan")
	}

	base := llmroute.Plan{Candidates: []string{"claude-tmux"}, Model: "sonnet"}
	overlaid := llmroute.ApplySoftOverlay(base, llmroute.Overlay{CLI: entry.CLI, Tier: entry.Tier}, prof)
	if len(overlaid.Candidates) == 0 || overlaid.Candidates[0] != "codex-tmux" {
		t.Errorf("dispatch chain = %v, want codex-tmux promoted to primary", overlaid.Candidates)
	}
	if overlaid.Model != "balanced" {
		t.Errorf("dispatch model = %q, want balanced (the clamped tier)", overlaid.Model)
	}
}

func TestReplayPlanFromResponse_LegacyResponseByteIdenticalDispatch(t *testing.T) {
	t.Parallel()
	raw := `[{"phase":"scout","run":true,"justification":"scout the work"},{"phase":"build","run":true,"justification":"do the work"}]`
	in := router.RouteInput{}
	floor := []string{}

	floored, _, err := ReplayPlanFromResponse(raw, in, floor)
	if err != nil {
		t.Fatalf("ReplayPlanFromResponse: %v", err)
	}
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"}}
	mrPlan, mrClamps := router.ClampPlanModelRouting(floored, func(string) *profiles.Profile { return prof }, (modelcatalog.Catalog{}).Lookup)
	if len(mrClamps) != 0 {
		t.Fatalf("legacy response must clamp nothing; clamps=%+v", mrClamps)
	}

	base := llmroute.Plan{Candidates: []string{"claude-tmux"}, Model: "sonnet"}
	for _, e := range mrPlan.Entries {
		overlaid := llmroute.ApplySoftOverlay(base, llmroute.Overlay{CLI: e.CLI, Tier: e.Tier}, prof)
		if len(overlaid.Candidates) != 1 || overlaid.Candidates[0] != "claude-tmux" || overlaid.Model != "sonnet" {
			t.Errorf("phase %s: dispatch = %+v, want byte-identical to the profile-static baseline (no overlay)", e.Phase, overlaid)
		}
	}
}

// Renders the production persona-path prompt against a realistic RouteInput
// fixture; must show the {cli,tier} schema and guardrail lines together, so a
// regression dropping either turns this red independent of the narrower unit
// tests in phase_advisor_tier_elicitation_test.go.
func TestComposePlanPrompt_LiveShapeReplayCarriesTierSchemaAndGuardrails(t *testing.T) {
	t.Parallel()
	in := baseRouteInput()
	in.GoalText = "harden the payment webhook against replay attacks"
	in.Catalog = []router.PhaseCard{
		{Name: "security-scan", Role: "evaluate", Optional: true,
			AllowedCLIs:       []string{"claude"},
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}},
	}
	p := NewPhaseAdvisor(nil, WithPersona("PERSONA BODY"))
	got := p.composePlanPrompt(in, "routing-plan.json")

	for _, want := range []string{`"cli":`, `"tier":"balanced"`, "allowed_clis: claude", "model_tier_envelope: {min: balanced"} {
		if !strings.Contains(got, want) {
			t.Errorf("live-shape replay prompt missing %q; got:\n%s", want, got)
		}
	}
}

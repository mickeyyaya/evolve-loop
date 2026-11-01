package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestReplayPlanFromResponse_OnlyTheTierOfAnAdvisorResponseReachesDispatch(t *testing.T) {
	t.Parallel()
	raw := `[{"phase":"build","run":true,"justification":"needs deep reasoning","cli":"codex","tier":"balanced"}]`
	in := router.RouteInput{}
	floor := []string{}

	floored, _, err := ReplayPlanFromResponse(raw, in, floor)
	if err != nil {
		t.Fatalf("ReplayPlanFromResponse: %v", err)
	}

	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude", "codex"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}}
	mrPlan, mrClamps := router.ClampPlanModelRouting(floored, func(string) *profiles.Profile { return prof })
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
	overlaid := llmroute.ApplySoftOverlay(base, llmroute.Overlay{Tier: entry.Tier}, prof)
	if len(overlaid.Candidates) != 1 || overlaid.Candidates[0] != "claude-tmux" {
		t.Errorf("dispatch chain = %v, want the routed claude-tmux: a stale \"cli\" in the advisor response never moves the driver", overlaid.Candidates)
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
	mrPlan, mrClamps := router.ClampPlanModelRouting(floored, func(string) *profiles.Profile { return prof })
	if len(mrClamps) != 0 {
		t.Fatalf("legacy response must clamp nothing; clamps=%+v", mrClamps)
	}

	base := llmroute.Plan{Candidates: []string{"claude-tmux"}, Model: "sonnet"}
	for _, e := range mrPlan.Entries {
		overlaid := llmroute.ApplySoftOverlay(base, llmroute.Overlay{Tier: e.Tier}, prof)
		if len(overlaid.Candidates) != 1 || overlaid.Candidates[0] != "claude-tmux" || overlaid.Model != "sonnet" {
			t.Errorf("phase %s: dispatch = %+v, want byte-identical to the profile-static baseline (no overlay)", e.Phase, overlaid)
		}
	}
}

func TestComposePlanPrompt_LiveShapeReplayCarriesTierSchemaAndGuardrails(t *testing.T) {
	t.Parallel()
	in := baseRouteInput()
	in.GoalText = "harden the payment webhook against replay attacks"
	in.Catalog = []router.PhaseCard{
		{Name: "security-scan", Role: "evaluate", Optional: true,
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}},
	}
	p := NewPhaseAdvisor(nil, WithPersona("PERSONA BODY"))
	got := p.composePlanPrompt(in, "routing-plan.json")

	for _, want := range []string{`"tier":"balanced"`, "model_tier_envelope: {min: balanced"} {
		if !strings.Contains(got, want) {
			t.Errorf("live-shape replay prompt missing %q; got:\n%s", want, got)
		}
	}
	for _, banned := range []string{`"cli":`, "allowed_clis"} {
		if strings.Contains(got, banned) {
			t.Errorf("live-shape replay prompt asks the advisor for a CLI (%q); got:\n%s", banned, got)
		}
	}
}

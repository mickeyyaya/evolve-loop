package router

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestClampPlanModelRouting_Matrix(t *testing.T) {
	t.Run("out-of-envelope-tier-above-max", func(t *testing.T) {
		prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "balanced"}}
		catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
			"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"balanced": "sonnet", "deep": "opus"}},
		}}
		plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "deep"}}}

		out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
		if len(clamps) != 1 {
			t.Fatalf("clamps = %+v, want exactly one", clamps)
		}
		if clamps[0].Phase != "build" {
			t.Errorf("clamp.Phase = %q, want build", clamps[0].Phase)
		}
		if out.Entries[0].Tier == "deep" {
			t.Errorf("out-of-envelope-above-max tier %q survived the clamp", out.Entries[0].Tier)
		}
	})

	t.Run("fast-below-min-clamps-up-to-balanced", func(t *testing.T) {
		prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}}
		catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
			"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"balanced": "sonnet", "deep": "opus"}},
		}}
		plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "fast"}}}

		out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
		if len(clamps) != 1 {
			t.Fatalf("clamps = %+v, want exactly one", clamps)
		}
		if clamps[0].Phase != "build" {
			t.Errorf("clamp.Phase = %q, want build", clamps[0].Phase)
		}
		if out.Entries[0].Tier != "balanced" {
			t.Errorf("fast-below-min entry.Tier = %q, want CLAMPED UP to balanced (operator low-model floor), not emptied", out.Entries[0].Tier)
		}
		if out.Entries[0].CLI != "claude" {
			t.Errorf("fast-below-min entry.CLI = %q, want unchanged (only the tier violated the floor)", out.Entries[0].CLI)
		}
	})

	t.Run("disallowed-cli", func(t *testing.T) {
		prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"}}
		plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "audit", Run: true, CLI: "mallory-cli"}}}

		out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), (modelcatalog.Catalog{}).Lookup)
		if len(clamps) != 1 {
			t.Fatalf("clamps = %+v, want exactly one", clamps)
		}
		if clamps[0].Phase != "audit" {
			t.Errorf("clamp.Phase = %q, want audit", clamps[0].Phase)
		}
		if out.Entries[0].CLI == "mallory-cli" {
			t.Errorf("disallowed CLI %q survived the clamp", out.Entries[0].CLI)
		}
	})

	t.Run("catalog-miss", func(t *testing.T) {
		prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "deep"}}
		plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "tdd", Run: true, CLI: "claude", Tier: "balanced"}}}

		out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), (modelcatalog.Catalog{}).Lookup)
		if len(clamps) != 1 {
			t.Fatalf("clamps = %+v, want exactly one", clamps)
		}
		if clamps[0].Phase != "tdd" {
			t.Errorf("clamp.Phase = %q, want tdd", clamps[0].Phase)
		}
		if out.Entries[0].CLI == "claude" && out.Entries[0].Tier == "balanced" {
			t.Errorf("catalog-miss entry %+v was not clamped", out.Entries[0])
		}
	})
}

func TestClampPlanModelRouting_NoRelaxationEvenWithJustification(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "balanced"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"balanced": "sonnet", "deep": "opus"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{
		Phase: "build", Run: true, CLI: "claude", Tier: "deep",
		Justification: "this cycle is unusually high-risk and deep reasoning is clearly justified",
	}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
	if len(clamps) != 1 {
		t.Fatalf("a persuasive justification must not suppress the clamp; clamps=%+v", clamps)
	}
	if out.Entries[0].Tier == "deep" {
		t.Errorf("justification text must never widen the envelope; entry=%+v", out.Entries[0])
	}
}

func TestRejectionsFromClamps_NamesPhaseAndReason(t *testing.T) {
	clamps := []Clamp{
		{Phase: "build", Rule: "model-routing-guardrail", Proposed: `build={cli:"mallory-cli",tier:""}`, Forced: "build={cli:,tier:} (profile default)"},
	}
	rej := RejectionsFromClamps(clamps)
	if len(rej) != 1 {
		t.Fatalf("rejections = %d, want 1", len(rej))
	}
	if rej[0].Phase != "build" {
		t.Errorf("rejection.Phase = %q, want build", rej[0].Phase)
	}
	if rej[0].Reason != "model-routing-guardrail" {
		t.Errorf("rejection.Reason = %q, want model-routing-guardrail", rej[0].Reason)
	}
	if !strings.Contains(rej[0].Detail, "mallory-cli") {
		t.Errorf("rejection.Detail must name the rejected proposal; got %q", rej[0].Detail)
	}
}

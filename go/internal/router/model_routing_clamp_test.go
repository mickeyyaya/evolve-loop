package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func profileFunc(p *profiles.Profile) func(string) *profiles.Profile {
	return func(string) *profiles.Profile { return p }
}

func TestClampPlanModelRouting_InBoundsHonored(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude", "codex"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"codex": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"balanced": "gpt-5-codex"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "codex", Tier: "balanced"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
	if len(clamps) != 0 {
		t.Fatalf("clamps = %+v, want none", clamps)
	}
	if out.Entries[0].CLI != "codex" || out.Entries[0].Tier != "balanced" {
		t.Errorf("entry mutated to %+v, want unchanged", out.Entries[0])
	}
}

func TestClampPlanModelRouting_ClampsOutOfEnvelopeTier(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "fast"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
	if len(clamps) != 1 {
		t.Fatalf("clamps = %+v, want exactly one", clamps)
	}
	if out.Entries[0].Tier == "fast" {
		t.Errorf("entry tier still %q after clamp", out.Entries[0].Tier)
	}
}

func TestClampPlanModelRouting_ClampsDisallowedCLI(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "mallory-cli"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), (modelcatalog.Catalog{}).Lookup)
	if len(clamps) != 1 {
		t.Fatalf("clamps = %+v, want exactly one", clamps)
	}
	if out.Entries[0].CLI == "mallory-cli" {
		t.Errorf("disallowed CLI %q survived the clamp", out.Entries[0].CLI)
	}
}

func TestClampPlanModelRouting_ClampsCatalogMiss(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "deep"}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "balanced"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), (modelcatalog.Catalog{}).Lookup)
	if len(clamps) != 1 {
		t.Fatalf("clamps = %+v, want exactly one", clamps)
	}
	if out.Entries[0].CLI == "claude" && out.Entries[0].Tier == "balanced" {
		t.Errorf("catalog-miss entry %+v was not clamped", out.Entries[0])
	}
}

func TestClampPlanModelRouting_CrossFamilyIsPreferenceNotReject(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux"}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"codex": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"balanced": "gpt-5-codex"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "codex", Tier: "balanced"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
	if len(clamps) != 0 {
		t.Errorf("cross-family proposal produced clamps %+v, want none", clamps)
	}
	if out.Entries[0].CLI != "codex" {
		t.Errorf("cross-family CLI forced to %q, want unchanged codex", out.Entries[0].CLI)
	}
}

func TestClampPlanModelRouting_SuffixedCLIHonoredViaBaseName(t *testing.T) {
	if got := policy.BaseCLI("claude-tmux"); got != "claude" {
		t.Fatalf("setup: policy.BaseCLI(%q) = %q, want claude", "claude-tmux", got)
	}
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "deep"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		// The catalog is keyed on the base family, never a driver-qualified name.
		"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"deep": "opus"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "scout", Run: true, CLI: "claude-tmux", Tier: "deep"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
	if len(clamps) != 0 {
		t.Fatalf("clamps = %+v, want none (suffixed CLI's base family resolves in the catalog)", clamps)
	}
	if out.Entries[0].CLI != "claude-tmux" || out.Entries[0].Tier != "deep" {
		t.Errorf("entry = %+v, want unchanged {cli:claude-tmux,tier:deep} (I1: honored entry keeps its original CLI string)", out.Entries[0])
	}
}

func TestClampPlanModelRouting_NilPlanAndNoProposal(t *testing.T) {
	if out, clamps := ClampPlanModelRouting(nil, profileFunc(nil), (modelcatalog.Catalog{}).Lookup); out != nil || clamps != nil {
		t.Errorf("nil plan => (%v, %v), want (nil, nil)", out, clamps)
	}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "scout", Run: true}}}
	out, clamps := ClampPlanModelRouting(plan, profileFunc(nil), (modelcatalog.Catalog{}).Lookup)
	if len(clamps) != 0 {
		t.Errorf("clamps = %+v, want none for an entry proposing neither cli nor tier", clamps)
	}
	if out.Entries[0].CLI != "" || out.Entries[0].Tier != "" {
		t.Errorf("entry = %+v, want unchanged empty CLI/Tier", out.Entries[0])
	}
}

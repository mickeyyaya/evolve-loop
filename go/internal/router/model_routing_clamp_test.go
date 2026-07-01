package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func profileFunc(p *profiles.Profile) func(string) *profiles.Profile {
	return func(string) *profiles.Profile { return p }
}

// TestClampPlanModelRouting_InBoundsHonored: an in-bounds, catalog-resolvable
// {cli,tier} proposal passes through unchanged with zero clamps.
func TestClampPlanModelRouting_InBoundsHonored(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude", "codex"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"codex": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"balanced": "gpt-5-codex"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "codex", Tier: "balanced"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog)
	if len(clamps) != 0 {
		t.Fatalf("clamps = %+v, want none", clamps)
	}
	if out.Entries[0].CLI != "codex" || out.Entries[0].Tier != "balanced" {
		t.Errorf("entry mutated to %+v, want unchanged", out.Entries[0])
	}
}

// TestClampPlanModelRouting_ClampsOutOfEnvelopeTier: a tier below the
// profile's envelope minimum is forced back and recorded as one clamp.
func TestClampPlanModelRouting_ClampsOutOfEnvelopeTier(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "fast"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog)
	if len(clamps) != 1 {
		t.Fatalf("clamps = %+v, want exactly one", clamps)
	}
	if out.Entries[0].Tier == "fast" {
		t.Errorf("entry tier still %q after clamp", out.Entries[0].Tier)
	}
}

// TestClampPlanModelRouting_ClampsDisallowedCLI: a CLI outside allowed_clis
// never survives unchanged.
func TestClampPlanModelRouting_ClampsDisallowedCLI(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "mallory-cli"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), modelcatalog.Catalog{})
	if len(clamps) != 1 {
		t.Fatalf("clamps = %+v, want exactly one", clamps)
	}
	if out.Entries[0].CLI == "mallory-cli" {
		t.Errorf("disallowed CLI %q survived the clamp", out.Entries[0].CLI)
	}
}

// TestClampPlanModelRouting_ClampsCatalogMiss: an otherwise-legal pair the
// live catalog cannot resolve is clamped away rather than reaching dispatch.
func TestClampPlanModelRouting_ClampsCatalogMiss(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "deep"}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "claude", Tier: "balanced"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), modelcatalog.Catalog{})
	if len(clamps) != 1 {
		t.Fatalf("clamps = %+v, want exactly one", clamps)
	}
	if out.Entries[0].CLI == "claude" && out.Entries[0].Tier == "balanced" {
		t.Errorf("catalog-miss entry %+v was not clamped", out.Entries[0])
	}
}

// TestClampPlanModelRouting_CrossFamilyIsPreferenceNotReject (B2): with no
// allowed_clis restriction configured, a cross-family CLI choice is legal by
// default — a clamp must not equate "different family" with "disallowed".
func TestClampPlanModelRouting_CrossFamilyIsPreferenceNotReject(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux"}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"codex": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"balanced": "gpt-5-codex"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, CLI: "codex", Tier: "balanced"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog)
	if len(clamps) != 0 {
		t.Errorf("cross-family proposal produced clamps %+v, want none", clamps)
	}
	if out.Entries[0].CLI != "codex" {
		t.Errorf("cross-family CLI forced to %q, want unchanged codex", out.Entries[0].CLI)
	}
}

// TestClampPlanModelRouting_NilPlanAndNoProposal covers the defensive nil-plan
// return and the "nothing proposed" no-op path (an entry with CLI==Tier=="").
func TestClampPlanModelRouting_NilPlanAndNoProposal(t *testing.T) {
	if out, clamps := ClampPlanModelRouting(nil, profileFunc(nil), modelcatalog.Catalog{}); out != nil || clamps != nil {
		t.Errorf("nil plan => (%v, %v), want (nil, nil)", out, clamps)
	}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "scout", Run: true}}}
	out, clamps := ClampPlanModelRouting(plan, profileFunc(nil), modelcatalog.Catalog{})
	if len(clamps) != 0 {
		t.Errorf("clamps = %+v, want none for an entry proposing neither cli nor tier", clamps)
	}
	if out.Entries[0].CLI != "" || out.Entries[0].Tier != "" {
		t.Errorf("entry = %+v, want unchanged empty CLI/Tier", out.Entries[0])
	}
}

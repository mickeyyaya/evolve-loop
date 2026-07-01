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

// TestClampPlanModelRouting_InBoundsHonored: an in-bounds, catalog-resolvable
// {cli,tier} proposal passes through unchanged with zero clamps.
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

// TestClampPlanModelRouting_ClampsOutOfEnvelopeTier: a tier below the
// profile's envelope minimum is forced back and recorded as one clamp.
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

// TestClampPlanModelRouting_ClampsDisallowedCLI: a CLI outside allowed_clis
// never survives unchanged.
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

// TestClampPlanModelRouting_ClampsCatalogMiss: an otherwise-legal pair the
// live catalog cannot resolve is clamped away rather than reaching dispatch.
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

// TestClampPlanModelRouting_CrossFamilyIsPreferenceNotReject (B2): with no
// allowed_clis restriction configured, a cross-family CLI choice is legal by
// default — a clamp must not equate "different family" with "disallowed".
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

// TestClampPlanModelRouting_SuffixedCLIHonoredViaBaseName (mr4b AC1, F2 fix):
// a suffixed CLI like "claude-tmux" whose BASE FAMILY ("claude", via
// policy.BaseCLI) resolves in the live catalog must be HONORED — not wrongly
// clamped as a catalog miss. Before the F2 fix, ClampPlanModelRouting passed
// the raw suffixed e.CLI to catalogLookup, which is keyed on the base family,
// so a valid suffixed proposal always missed. The suffixed CLI string itself
// (e.CLI) must survive UNCHANGED on the honored path (api-contract Invariant
// I1: normalization is catalogLookup-only, never a rewrite of e.CLI).
func TestClampPlanModelRouting_SuffixedCLIHonoredViaBaseName(t *testing.T) {
	if got := policy.BaseCLI("claude-tmux"); got != "claude" {
		t.Fatalf("setup: policy.BaseCLI(%q) = %q, want claude", "claude-tmux", got)
	}
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "deep"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		// Catalog is keyed on the BASE family "claude", never on a driver-
		// qualified name — exactly the mismatch F2 exploited.
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

// TestClampPlanModelRouting_NilPlanAndNoProposal covers the defensive nil-plan
// return and the "nothing proposed" no-op path (an entry with CLI==Tier=="").
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

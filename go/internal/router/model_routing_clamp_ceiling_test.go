package router

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestClampPlanModelRouting_ClampsAboveCeilingTier(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"},
		ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "balanced"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"fast": "haiku", "balanced": "sonnet", "deep": "opus"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "memo", Run: true, CLI: "claude", Tier: "deep"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
	if len(clamps) != 1 {
		t.Fatalf("clamps = %+v, want exactly one (the ceiling)", clamps)
	}
	if !strings.Contains(clamps[0].Forced, "ceiling") {
		t.Errorf("clamp record should name the ceiling; got %+v", clamps[0])
	}
	if out.Entries[0].Tier != "balanced" {
		t.Errorf("tier = %q, want %q (clamped down to envelope max)", out.Entries[0].Tier, "balanced")
	}
	if out.Entries[0].CLI != "claude" {
		t.Errorf("CLI mutated to %q; only the tier violated a bound", out.Entries[0].CLI)
	}
}

func TestClampPlanModelRouting_EnvelopelessDeepStaysLegal(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"deep": "opus"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "audit", Run: true, CLI: "claude", Tier: "deep"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
	if len(clamps) != 0 {
		t.Fatalf("clamps = %+v, want none (below the universal Max)", clamps)
	}
	if out.Entries[0].Tier != "deep" {
		t.Errorf("tier = %q, want deep (unchanged)", out.Entries[0].Tier)
	}
}

func TestClampPlanModelRouting_EnvelopelessTopStaysLegal(t *testing.T) {
	prof := &profiles.Profile{CLI: "claude-tmux", AllowedCLIs: []string{"claude"}}
	catalog := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{
		"claude": {Source: modelcatalog.SourceLive, TierModels: map[string]string{"top": "opus"}},
	}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "audit", Run: true, CLI: "claude", Tier: "top"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof), catalog.Lookup)
	if len(clamps) != 0 {
		t.Fatalf("clamps = %+v, want none (top IS the universal ceiling, not above it)", clamps)
	}
	if out.Entries[0].Tier != "top" {
		t.Errorf("tier = %q, want top (unchanged — envelope-less profiles must keep the frontier tier)", out.Entries[0].Tier)
	}
}

package router

// model_routing_clamp_ceiling_test.go — L5 (static/dynamic boundary review
// 2026-07-16): the routing clamp enforced only a FLOOR (clamp-up to the
// envelope Min); a tier proposal ABOVE the envelope Max sailed through — an
// advisor could route a memo-class phase (max balanced) onto deep, a
// cost/quota leak (the same pressure class behind the quota storms). These
// tests pin the ceiling: above-Max clamps DOWN to Max, and the compiled
// universal envelope (Max "top" — the HIGHEST TierRank, above deep) keeps
// envelope-less profiles unaffected: no behavior change for the 72/91
// profiles without an explicit envelope, including advisor-proposed "top".

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// TestClampPlanModelRouting_ClampsAboveCeilingTier: deep proposed against an
// explicit max=balanced envelope is forced DOWN to balanced (CLI untouched —
// only the tier violated a bound), recorded as one clamp naming the ceiling.
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

// TestClampPlanModelRouting_EnvelopelessDeepStaysLegal: a profile with NO
// explicit envelope keeps accepting deep proposals — the ceiling introduces
// zero behavior change for the envelope-less majority.
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

// TestClampPlanModelRouting_EnvelopelessTopStaysLegal — go-reviewer HIGH
// regression pin (2026-07-16): "top" is TierRank's HIGHEST tier (above deep)
// and a live advisor-proposable value (sanitizeAdvisorTier keeps it). The
// universal envelope's Max MUST be "top", or activating the ceiling silently
// forecloses the frontier tier for every envelope-less profile (72/91) — the
// exact false-comfort this test would have caught the first time.
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

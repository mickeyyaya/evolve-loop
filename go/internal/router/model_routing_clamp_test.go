package router

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func profileFunc(p *profiles.Profile) func(string) *profiles.Profile {
	return func(string) *profiles.Profile { return p }
}

func TestClampPlanModelRouting_InBoundsHonored(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, Tier: "balanced"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof))
	if len(clamps) != 0 {
		t.Fatalf("clamps = %+v, want none", clamps)
	}
	if out.Entries[0].Tier != "balanced" {
		t.Errorf("entry mutated to %+v, want unchanged", out.Entries[0])
	}
}

func TestClampPlanModelRouting_ClampsOutOfEnvelopeTier(t *testing.T) {
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "balanced", Max: "deep"}}
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true, Tier: "fast"}}}

	out, clamps := ClampPlanModelRouting(plan, profileFunc(prof))
	if len(clamps) != 1 {
		t.Fatalf("clamps = %+v, want exactly one", clamps)
	}
	if out.Entries[0].Tier != "balanced" {
		t.Errorf("entry tier = %q after clamp, want balanced", out.Entries[0].Tier)
	}
	if strings.Contains(clamps[0].Proposed+clamps[0].Forced, "cli") {
		t.Errorf("the clamp record names a CLI: %+v", clamps[0])
	}
}

func TestClampPlanModelRouting_NilPlanAndNoProposal(t *testing.T) {
	if out, clamps := ClampPlanModelRouting(nil, profileFunc(nil)); out != nil || clamps != nil {
		t.Errorf("nil plan => (%v, %v), want (nil, nil)", out, clamps)
	}
	loaded := 0
	plan := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "scout", Run: true}}}
	out, clamps := ClampPlanModelRouting(plan, func(string) *profiles.Profile { loaded++; return nil })
	if len(clamps) != 0 || loaded != 0 {
		t.Errorf("clamps = %+v, profile loads = %d; want none for an entry with no tier", clamps, loaded)
	}
	if out.Entries[0].Tier != "" {
		t.Errorf("entry = %+v, want unchanged empty tier", out.Entries[0])
	}
	unprofiled := &PhasePlan{Entries: []PhasePlanEntry{{Phase: "minted", Run: true, Tier: "fast"}}}
	if out, clamps := ClampPlanModelRouting(unprofiled, profileFunc(nil)); len(clamps) != 0 || out.Entries[0].Tier != "fast" {
		t.Errorf("a phase with no profile has no envelope to clamp to: clamps=%+v entry=%+v", clamps, out.Entries[0])
	}
}

func TestPhasePlanEntry_TheAdvisorCannotNameACLI(t *testing.T) {
	var entries []PhasePlanEntry
	raw := `[{"phase":"build","run":true,"cli":"codex-tmux","tier":"deep","mint":{"prompt":"p","tier":"deep","cli":"codex"}}]`
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		t.Fatal(err)
	}
	back, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(back), "cli") || !strings.Contains(string(back), `"tier":"deep"`) {
		t.Fatalf("a plan entry keeps only the tier, never a CLI: %s", back)
	}
}

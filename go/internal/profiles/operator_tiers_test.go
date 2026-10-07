package profiles

import (
	"slices"
	"testing"
)

func realTreeProfiles(t *testing.T) map[string]Profile {
	t.Helper()
	loader := NewFromDir(RealProfilesDir(t))
	names, err := loader.List()
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]Profile, len(names))
	for _, name := range names {
		p, err := loader.Get(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out[name] = p
	}
	return out
}

func TestNoTrackedProfileDefaultsToTheFastTier(t *testing.T) {
	for name, p := range realTreeProfiles(t) {
		if p.ModelTierDefault == "fast" {
			t.Errorf("%s defaults to fast (Gemini 3.8 Flash Low); the operator runs every non-deep, non-top phase at balanced (Flash High)", name)
		}
		if gaps := fastTierEnvelopeGaps(p); len(gaps) > 0 {
			t.Errorf("%s: envelope %v lets the advisor or a tier step-down put it on Flash Low; want balanced at least (no profile is exempt)", name, gaps)
		}
	}
}

func fastTierEnvelopeGaps(p Profile) []string {
	env := p.ModelTierEnvelope
	if env == nil {
		return nil
	}
	var gaps []string
	for field, tier := range map[string]string{"min": env.Min, "default": env.Default, "max": env.Max} {
		if tier == "fast" || tier == "haiku" {
			gaps = append(gaps, field+"="+tier)
		}
	}
	slices.Sort(gaps)
	return gaps
}

func TestFastTierEnvelopeGaps_NamesEveryFastBoundAndPassesABalancedEnvelope(t *testing.T) {
	cases := map[string]struct {
		env  *ModelTierEnvelope
		want []string
	}{
		"no envelope":        {nil, nil},
		"balanced to deep":   {&ModelTierEnvelope{Min: "balanced", Default: "balanced", Max: "deep"}, nil},
		"fast floor":         {&ModelTierEnvelope{Min: "fast", Default: "balanced", Max: "balanced"}, []string{"min=fast"}},
		"vendor haiku floor": {&ModelTierEnvelope{Min: "haiku", Default: "fast", Max: "balanced"}, []string{"default=fast", "min=haiku"}},
	}
	for name, tc := range cases {
		if got := fastTierEnvelopeGaps(Profile{ModelTierEnvelope: tc.env}); !slices.Equal(got, tc.want) {
			t.Errorf("%s: gaps = %v, want %v", name, got, tc.want)
		}
	}
}

func TestTheFormerFastPhasesKeepABalancedFloor(t *testing.T) {
	profs := realTreeProfiles(t)
	for _, name := range []string{
		"behavior-baseline", "changelog-sync", "close-checklist", "dependency-map", "doc-sync",
		"flake-rerun-scan", "locale-format-check", "runbook-draft", "scope-baseline", "telemetry-coverage-check",
	} {
		p, found := profs[name]
		if !found || p.ModelTierDefault != "balanced" {
			t.Errorf("%s: default %q, want balanced", name, p.ModelTierDefault)
		}
		if env := p.ModelTierEnvelope; env != nil && env.Min != "balanced" {
			t.Errorf("%s: envelope min %q lets the advisor lower it back to Flash Low; want balanced", name, env.Min)
		}
	}
}

func TestEveryProfileChainIsWithinAllowedCLIs(t *testing.T) {
	for name, p := range realTreeProfiles(t) {
		for _, cli := range append([]string{p.CLI}, p.CLIFallback...) {
			if cli != "" && !p.AllowsFamily(cli) {
				t.Errorf("%s routes %s, outside its own allowed_clis %v", name, cli, p.AllowedCLIs)
			}
		}
	}
}

func TestTheBuilderAndTheTesterMayRunOnAgyAndOnAgyOwnedClaude(t *testing.T) {
	profs := realTreeProfiles(t)
	for _, name := range []string{"builder", "tester"} {
		p := profs[name]
		got := p.AllowedFamilies()
		for _, family := range []string{"agy", "agy-claude", "claude"} {
			if got != nil && !slices.Contains(got, family) {
				t.Errorf("%s allowed_clis %v lacks %s: it builds on agy at balanced and escalates to agy-owned Claude, then Claude Code", name, got, family)
			}
		}
	}
}

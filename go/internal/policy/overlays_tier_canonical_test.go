package policy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestResolveOverlays_TierSelectorsMatchTheCanonicalTier(t *testing.T) {
	cases := []struct {
		selector string
		tier     string
		want     bool
	}{
		{"deep", "deep", true},
		{"deep", "opus", true},
		{"deep", "claude-opus-4-1", true},
		{"deep", " Deep ", true},
		{"deep", "sonnet", false},
		{"deep", "top", false},
		{"deep", "", false},
		{"deep", "gpt-5.6-sol", false},
		{"fast", "haiku", true},
		{"fast", "claude-haiku-4-5", true},
		{"balanced", "claude-sonnet-4-5", true},
		{"opus", "deep", true},
		{"opus", "claude-opus-4", true},
		{"haiku", "fast", true},
		{"haiku", "balanced", false},
		{"nonsense", "zzz", false},
		{"nonsense", "", false},
		{"nonsense", "nonsense", true},
		{"*", "zzz", true},
		{"d*", "opus", false},
		{"d*", "deep", true},
		{"*opus*", "deep", false},
		{"*opus*", "claude-opus-4-1", true},
		{"claude-opus-*", "opus", false},
		{"*sonnet*", "balanced", false},
		{`\opus`, "deep", false},
	}
	for _, tc := range cases {
		t.Run(tc.selector+"/"+tc.tier, func(t *testing.T) {
			pol := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
				{Tiers: []string{tc.selector}, Skills: []string{"probe"}},
			}}}
			got := len(pol.ResolveOverlays(policy.OverlayDispatch{Tier: tc.tier})) == 1
			if got != tc.want {
				t.Errorf("tiers [%s] vs dispatch tier %q matched=%v, want %v", tc.selector, tc.tier, got, tc.want)
			}
		})
	}
}

func TestResolveOverlays_CompiledDefaultReachesConcreteOpusModels(t *testing.T) {
	var pol policy.Policy
	for _, model := range []string{"opus", "claude-opus-4-1"} {
		if got := pol.ResolveOverlays(policy.DispatchFromPhaseRequest("advisor", "claude-tmux", model, model)); !reflect.DeepEqual(got, []string{"fable"}) {
			t.Errorf("compiled default for model %q = %v, want [fable]", model, got)
		}
	}
	if got := pol.ResolveOverlays(policy.DispatchFromPhaseRequest("advisor", "claude-tmux", "sonnet", "sonnet")); len(got) != 0 {
		t.Errorf("compiled default for model sonnet = %v, want none", got)
	}
}

func TestNonCanonicalOverlayTierSelectors(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want []string
	}{
		{"absent block uses the canonical compiled default", policy.Policy{}, nil},
		{"empty rules", policy.Policy{Overlays: &policy.OverlaysPolicy{}}, nil},
		{"canonical, alias, model id and glob selectors", policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
			{Tiers: []string{"deep", "top", "opus", "claude-haiku-4-5", "*", "d*"}},
		}}}, nil},
		{"non-canonical selectors deduped in first-seen order", policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
			{Tiers: []string{"deep", "nonsense"}},
			{Tiers: []string{"gpt-5.6-sol", "nonsense", "x*"}},
		}}}, []string{"nonsense", "gpt-5.6-sol", "x*"}},
		{"globs naming an alias substring match no canonical tier name", policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
			{Tiers: []string{"*opus*", "?op", "claude-opus-*"}},
		}}}, []string{"*opus*", "claude-opus-*"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pol.NonCanonicalOverlayTierSelectors(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NonCanonicalOverlayTierSelectors() = %v, want %v", got, tc.want)
			}
		})
	}
}

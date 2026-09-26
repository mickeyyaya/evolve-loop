package policy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestResolveOverlays_AbsentBlockUsesCompiledDefault(t *testing.T) {
	var pol policy.Policy

	deep := pol.ResolveOverlays(policy.OverlayDispatch{Tier: "deep"})
	if !reflect.DeepEqual(deep, []string{"fable"}) {
		t.Errorf("ResolveOverlays(tier=deep) = %v, want [fable] (compiled default)", deep)
	}

	top := pol.ResolveOverlays(policy.OverlayDispatch{Tier: "top"})
	if !reflect.DeepEqual(top, []string{"fable"}) {
		t.Errorf("ResolveOverlays(tier=top) = %v, want [fable] (compiled default)", top)
	}

	fast := pol.ResolveOverlays(policy.OverlayDispatch{Tier: "fast"})
	if len(fast) != 0 {
		t.Errorf("ResolveOverlays(tier=fast) = %v, want none — the compiled default is deep/top only", fast)
	}
}

func TestResolveOverlays_ExplicitEmptyRulesDisablesDefault(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{}}}

	got := pol.ResolveOverlays(policy.OverlayDispatch{Tier: "deep"})
	if len(got) != 0 {
		t.Errorf("ResolveOverlays with explicit empty rules = %v, want none (operator opt-out honored)", got)
	}
}

func TestResolveOverlays_MultiRuleUnionDedupStableOrder(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{Tiers: []string{"deep", "top"}, Skills: []string{"fable", "engineering-craft"}},
		{Phases: []string{"audit"}, Skills: []string{"audit-discipline", "fable"}},
	}}}

	got := pol.ResolveOverlays(policy.OverlayDispatch{Tier: "deep", Phase: "audit"})
	want := []string{"fable", "engineering-craft", "audit-discipline"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ResolveOverlays union = %v, want %v (deduped, stable first-seen order)", got, want)
	}
}

func TestResolveOverlays_SelectorMatrix(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{Phases: []string{"audit"}, Skills: []string{"audit-discipline"}},
		{Models: []string{"gpt-*"}, Skills: []string{"codex-house-style"}},
	}}}

	cases := []struct {
		name string
		d    policy.OverlayDispatch
		want []string
	}{
		{"audit phase, any cli/model", policy.OverlayDispatch{Phase: "audit", CLI: "claude-tmux", Model: "sonnet"}, []string{"audit-discipline"}},
		{"audit phase via codex", policy.OverlayDispatch{Phase: "audit", CLI: "codex", Model: "gpt-5"}, []string{"audit-discipline", "codex-house-style"}},
		{"non-audit phase, gpt model", policy.OverlayDispatch{Phase: "build", CLI: "codex", Model: "gpt-5"}, []string{"codex-house-style"}},
		{"non-audit phase, non-gpt model", policy.OverlayDispatch{Phase: "build", CLI: "claude-tmux", Model: "sonnet"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pol.ResolveOverlays(tc.d)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveOverlays(%+v) = %v, want %v", tc.d, got, tc.want)
			}
		})
	}
}

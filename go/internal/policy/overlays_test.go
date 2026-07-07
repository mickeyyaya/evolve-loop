package policy_test

// Overlays resolver — cycle-609 task skill-overlays-bridge-layer.
//
// Nil-able-pointer + resolver config idiom (ObserverPolicy exemplar,
// policy.go:483-539): Policy.Overlays == nil ⇒ the compiled default
// {tiers:[deep,top]}->[fable] applies; a non-nil block with an empty
// Rules slice is an explicit operator opt-out (zero overlays, not the
// default); a non-nil block with rules resolves the UNION of every
// matching rule's skills, deduped, in stable (first-seen) order.
//
// This is RED: internal/policy has no OverlaysPolicy/OverlayRule type and
// no ResolveOverlays method yet — Builder implements per the requeued spec
// (.evolve/inbox/2026-07-07T19-30-00Z-skill-overlays-bridge-layer-requeue.json).

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// TestResolveOverlays_AbsentBlockUsesCompiledDefault: Policy{} (Overlays
// nil) resolving a deep-tier dispatch must return exactly the compiled
// default skill set, per the spec's "COMPILED DEFAULT:
// [{tiers:[deep,top], skills:[fable]}]".
func TestResolveOverlays_AbsentBlockUsesCompiledDefault(t *testing.T) {
	var pol policy.Policy // Overlays field zero-value (nil pointer)

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

// TestResolveOverlays_ExplicitEmptyRulesDisablesDefault: an operator who
// sets overlays.rules: [] explicitly opts OUT of the compiled default —
// this must be distinguishable from "block absent" (the nil-able-pointer
// contract every other *Policy field in this file already follows).
func TestResolveOverlays_ExplicitEmptyRulesDisablesDefault(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{}}}

	got := pol.ResolveOverlays(policy.OverlayDispatch{Tier: "deep"})
	if len(got) != 0 {
		t.Errorf("ResolveOverlays with explicit empty rules = %v, want none (operator opt-out honored)", got)
	}
}

// TestResolveOverlays_MultiRuleUnionDedupStableOrder: two rules that both
// match the same dispatch must union their skills, deduped, in stable
// (first rule's skill order wins ties) order — not implementation-detail
// map iteration order.
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

// TestResolveOverlays_SelectorMatrix pins the per-dimension selector
// semantics from the spec's acceptance list: a phases-only rule matches
// on ANY cli/model for that phase; a models-glob rule matches only the
// named driver family; an empty selector dimension is a wildcard; a
// fast/balanced dispatch against only the compiled default gets nothing.
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

package policy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCompiledDefaultOverlays_CodeReviewSimplifyReachesOnlyCodeSourceWriters(t *testing.T) {
	var pol policy.Policy
	code := map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode}
	document := map[string]string{config.SignalDeliverableKind: config.DeliverableKindDocument}
	cases := []struct {
		name string
		d    policy.OverlayDispatch
		want []string
	}{
		{"balanced code build", policy.OverlayDispatch{Phase: "build", Tier: "balanced", WritesSource: true, Signals: code}, []string{"engineering-craft", "code-review-simplify", "docs-write"}},
		{"balanced code tdd", policy.OverlayDispatch{Phase: "tdd", Tier: "balanced", WritesSource: true, Signals: code}, []string{"engineering-craft", "code-review-simplify", "docs-write"}},
		{"balanced code audit", policy.OverlayDispatch{Phase: "audit", Tier: "balanced", Signals: code}, nil},
		{"document audit keeps its persona and the document skill", policy.OverlayDispatch{Phase: "audit", Tier: "balanced", Signals: document}, []string{"solution-audit", "docs-write"}},
		{"document build keeps its persona and the document skill", policy.OverlayDispatch{Phase: "build", Tier: "balanced", WritesSource: true, Signals: document}, []string{"solution-build", "docs-write"}},
		{"balanced code scout", policy.OverlayDispatch{Phase: "scout", Tier: "balanced", Signals: code}, nil},
		{"balanced code triage", policy.OverlayDispatch{Phase: "triage", Tier: "balanced", Signals: code}, nil},
		{"decision-only debugger", policy.OverlayDispatch{Phase: "debugger", Tier: "balanced", Signals: code}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pol.ResolveOverlays(tc.d); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveOverlays(%+v) = %v, want %v", tc.d, got, tc.want)
			}
		})
	}
}

func TestSelfReviewSkill_IsTheSkillTheCompiledWriterRuleLoadsBeforeTheDocumentSkill(t *testing.T) {
	var pol policy.Policy
	code := map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode}
	got := pol.ResolveOverlays(policy.OverlayDispatch{Phase: "build", Tier: "balanced", WritesSource: true, Signals: code})
	if len(got) < 2 || got[len(got)-2] != policy.SelfReviewSkill {
		t.Errorf("a code build resolves %v; the skill before the last must be policy.SelfReviewSkill (%q), the name the runner's self-review check reads", got, policy.SelfReviewSkill)
	}
}

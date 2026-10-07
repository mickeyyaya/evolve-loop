package policy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCompiledDefaultOverlays_TheCodeReviewPhaseLoadsTheStandardTheRubricsAndTheIndex(t *testing.T) {
	var pol policy.Policy
	code := map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode}
	document := map[string]string{config.SignalDeliverableKind: config.DeliverableKindDocument}
	review := []string{"engineering-craft", policy.SelfReviewSkill, "architecture-review", "quality-index"}
	cases := []struct {
		name string
		d    policy.OverlayDispatch
		want []string
	}{
		{"deep code review keeps fable first", policy.OverlayDispatch{Phase: "code-review", Tier: "deep", Signals: code}, append([]string{"fable"}, review...)},
		{"balanced code review", policy.OverlayDispatch{Phase: "code-review", Tier: "balanced", Signals: code}, review},
		{"a document cycle never reviews code", policy.OverlayDispatch{Phase: "code-review", Tier: "balanced", Signals: document}, nil},
		{"the audit stays free of review skills", policy.OverlayDispatch{Phase: "audit", Tier: "balanced", Signals: code}, nil},
		{"the build keeps only the writer rule", policy.OverlayDispatch{Phase: "build", Tier: "balanced", WritesSource: true, Signals: code}, []string{"engineering-craft", policy.SelfReviewSkill}},
		{"another evaluator gets nothing", policy.OverlayDispatch{Phase: "adversarial-review", Tier: "balanced", Signals: code}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pol.ResolveOverlays(tc.d); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveOverlays(%+v) = %v, want %v", tc.d, got, tc.want)
			}
		})
	}
}

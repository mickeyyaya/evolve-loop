package policy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCompiledDefaultOverlays_EngineeringCraftOnEveryCodeSourceWriterAtAnyTier(t *testing.T) {
	var pol policy.Policy
	code := map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode}
	cases := []struct {
		name string
		d    policy.OverlayDispatch
		want []string
	}{
		{"balanced build", policy.OverlayDispatch{Phase: "build", Tier: "balanced", WritesSource: true, Signals: code}, []string{"engineering-craft", "code-review-simplify"}},
		{"tdd at a concrete model tier", policy.OverlayDispatch{Phase: "tdd", Tier: "sonnet", WritesSource: true, Signals: code}, []string{"engineering-craft", "code-review-simplify"}},
		{"conflict-resolving debugger at opus", policy.OverlayDispatch{Phase: "debugger", Tier: "opus", WritesSource: true, Signals: code}, []string{"fable", "engineering-craft", "code-review-simplify"}},
		{"fast user phase that writes source", policy.OverlayDispatch{Phase: "test-amplification", Tier: "fast", WritesSource: true, Signals: code}, []string{"engineering-craft", "code-review-simplify"}},
		{"deep build keeps fable first", policy.OverlayDispatch{Phase: "build", Tier: "deep", WritesSource: true, Signals: code}, []string{"fable", "engineering-craft", "code-review-simplify"}},
		{"top tdd keeps fable first", policy.OverlayDispatch{Phase: "tdd", Tier: "top", WritesSource: true, Signals: code}, []string{"fable", "engineering-craft", "code-review-simplify"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pol.ResolveOverlays(tc.d); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveOverlays(%+v) = %v, want %v", tc.d, got, tc.want)
			}
		})
	}
}

func TestCompiledDefaultOverlays_EngineeringCraftNeverReachesAReadOnlyOrDocumentDispatch(t *testing.T) {
	var pol policy.Policy
	code := map[string]string{config.SignalDeliverableKind: config.DeliverableKindCode}
	document := map[string]string{config.SignalDeliverableKind: config.DeliverableKindDocument}
	cases := []struct {
		name string
		d    policy.OverlayDispatch
		want []string
	}{
		{"balanced scout", policy.OverlayDispatch{Phase: "scout", Tier: "balanced", Signals: code}, nil},
		{"balanced triage", policy.OverlayDispatch{Phase: "triage", Tier: "balanced", Signals: code}, nil},
		{"audit at opus", policy.OverlayDispatch{Phase: "audit", Tier: "opus", Signals: code}, []string{"fable"}},
		{"decision-only debugger at opus", policy.OverlayDispatch{Phase: "debugger", Tier: "opus", Signals: code}, []string{"fable"}},
		{"document build", policy.OverlayDispatch{Phase: "build", Tier: "balanced", WritesSource: true, Signals: document}, []string{"solution-build"}},
		{"writer without signals", policy.OverlayDispatch{Phase: "build", Tier: "balanced", WritesSource: true}, nil},
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

func TestResolveOverlays_WritesSourceSelectorMatchesOnlyWriters(t *testing.T) {
	pol := policy.Policy{Overlays: &policy.OverlaysPolicy{Rules: []policy.OverlayRule{
		{WritesSource: true, Skills: []string{"writer-lens"}},
		{Phases: []string{"build"}, Skills: []string{"build-lens"}},
	}}}

	writer := pol.ResolveOverlays(policy.OverlayDispatch{Phase: "build", WritesSource: true})
	if !reflect.DeepEqual(writer, []string{"writer-lens", "build-lens"}) {
		t.Errorf("writer build = %v, want [writer-lens build-lens]", writer)
	}
	reader := pol.ResolveOverlays(policy.OverlayDispatch{Phase: "build"})
	if !reflect.DeepEqual(reader, []string{"build-lens"}) {
		t.Errorf("read-only build = %v, want [build-lens]: a writes_source rule must not fire, an unset one stays a wildcard", reader)
	}
}

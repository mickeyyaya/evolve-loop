package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestTddPinned_TheRegistryRuleReleasesADocumentAndATrivialCycle(t *testing.T) {
	cfg := config.RoutingConfig{Conditional: map[string]config.CondRule{"tdd": config.DefaultTddRule()}}
	for _, tc := range []struct {
		name string
		sig  RoutingSignals
		want bool
	}{
		{"undeclared kind keeps the pin", RoutingSignals{}, true},
		{"a document releases it", RoutingSignals{Triage: TriageSignals{Present: true, CycleSize: "medium", DeliverableKind: DeliverableKindDocument}}, false},
		{"a trivial cycle releases it", RoutingSignals{Triage: TriageSignals{Present: true, CycleSize: "trivial"}}, false},
	} {
		if got := TddPinned(cfg, tc.sig); got != tc.want {
			t.Errorf("%s: TddPinned = %v, want %v", tc.name, got, tc.want)
		}
	}
	if !TddPinned(config.RoutingConfig{}, RoutingSignals{Triage: TriageSignals{Present: true, DeliverableKind: DeliverableKindDocument}}) {
		t.Error("with no conditional rule tdd must stay pinned, the more mandatory side")
	}
}

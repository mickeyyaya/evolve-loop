package core

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestOrchestratorRoutingStrategy_IsTheWiredStrategy(t *testing.T) {
	t.Parallel()
	strategy := router.LLMProposal{}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithRouting(config.RoutingConfig{}, strategy))
	if got := o.RoutingStrategy(); got != router.RoutingStrategy(strategy) {
		t.Fatalf("RoutingStrategy() = %#v, want the wired %#v", got, strategy)
	}
}

func TestOrchestratorRoutingConfig_IsTheWiredConfig(t *testing.T) {
	t.Parallel()
	cfg := config.RoutingConfig{Order: []string{"scout", "build", "memo"}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithRouting(cfg, nil))
	if got := o.RoutingConfig().Order; !slices.Equal(got, cfg.Order) {
		t.Fatalf("RoutingConfig().Order = %v, want %v", got, cfg.Order)
	}
}

func TestOrchestratorShipFloor_IsTheOverrideOrTheDefault(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithShipFloor([]string{"audit", "evaluate"}))
	if got := o.ShipFloor(); !slices.Equal(got, []string{"audit", "evaluate"}) {
		t.Fatalf("ShipFloor() = %v, want the override", got)
	}
	plain := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if got := plain.ShipFloor(); !slices.Equal(got, router.DefaultShipFloor()) {
		t.Fatalf("ShipFloor() = %v, want the default %v", got, router.DefaultShipFloor())
	}
}

func TestOrchestratorContractGateReviewer_FindsTheGateInTheChain(t *testing.T) {
	t.Parallel()
	gate := gateMarker{on: true}
	for _, tc := range []struct {
		name string
		opts []Option
		want DeliverableReviewer
	}{
		{"no reviewer", nil, nil},
		{"gate alone", []Option{WithReviewer(gate)}, gate},
		{"gate nested in a chain", []Option{WithReviewer(ChainReviewers(gateMarker{on: false}, gate))}, gate},
		{"gate switched off", []Option{WithReviewer(ChainReviewers(gateMarker{on: false}))}, nil},
	} {
		o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), tc.opts...)
		if got := o.ContractGateReviewer(); got != tc.want {
			t.Errorf("%s: ContractGateReviewer() = %#v, want %#v", tc.name, got, tc.want)
		}
	}
}

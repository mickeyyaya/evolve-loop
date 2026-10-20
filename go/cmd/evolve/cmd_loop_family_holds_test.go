package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestOrchestratorPool_TheAdmitRecordsAFamilyHoldThePoolNames(t *testing.T) {
	t.Parallel()
	b, _, _ := supplyCoordinator(t, t.TempDir())
	b.loadGraph = coreRouterGraph
	fc := policy.FleetConfig{Count: 4, MinLanes: 4, PlanSource: "triage", Orchestrator: enforceOrchestrator}

	p := b.orchestratorPool(fc, nil, 1)
	if p.FamilyHolds == nil {
		t.Fatal("pool family holds = nil, want the holds the admission records into")
	}
	got := p.Admit([]fleet.Todo{{ID: "ctx-cap", Files: []string{"docs/a.md"}}}, []fleet.Todo{{ID: "ctx-telemetry", Files: []string{"docs/b.md"}}})

	if got != -1 || p.FamilyHolds.Summary() != "ctx-telemetry (family:ctx, behind ctx-cap)" {
		t.Fatalf("admit = %d, holds = %q: the production admission must record the family hold into the pool's holds", got, p.FamilyHolds.Summary())
	}
}

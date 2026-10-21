package main

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func coreRouterGraph(context.Context, string, int) (*admission.ModuleGraph, error) {
	listing := admission.ModulePath + "/internal/core|" + admission.ModulePath + "/internal/router\n" +
		admission.ModulePath + "/internal/router|\n" +
		admission.ModulePath + "/cmd/evolve|" + admission.ModulePath + "/internal/core " + admission.ModulePath + "/internal/router\n"
	return admission.NewModuleGraph(listing, 120)
}

func coreAndRouterTodos() (running, candidate fleet.Todo) {
	return fleet.Todo{ID: "core-lane", Files: []string{"go/internal/core/state.go"}},
		fleet.Todo{ID: "router-todo", Files: []string{"go/internal/router/router.go"}}
}

func TestCellAdmission_AtShadowTheReachRuleHoldsAndThePoolAdmissionRecordsTheHolder(t *testing.T) {
	t.Parallel()
	for name, admissionStage := range map[string]string{"an unset key": "", "the shadow key": "shadow"} {
		t.Run(name, func(t *testing.T) {
			b, evolveDir, _ := supplyCoordinator(t, t.TempDir())
			b.loadGraph = coreRouterGraph
			oc := enforceOrchestrator
			oc.Admission = admissionStage
			running, candidate := coreAndRouterTodos()

			got := b.cellAdmission(oc, 1, nil, nil)([]fleet.Todo{running}, []fleet.Todo{candidate})

			if got != -1 {
				t.Fatalf("admit = %d, want -1: the orchestrator stage is enforce but the admission key is %q, so the reach rule holds", got, admissionStage)
			}
			events := supplyEvents(t, b, evolveDir, orchestrator.CodeAdmissionShadow)
			if len(events) != 1 || events[0].Fields["todo"] != "router-todo" || events[0].Fields["held_by"] != "core-lane" {
				t.Fatalf("shadow events = %+v, want router-todo recorded with core-lane as its holder", events)
			}
		})
	}
}

func TestCellAdmission_AtEnforceACoreLaneDoesNotHoldARouterTodo(t *testing.T) {
	t.Parallel()
	b, evolveDir, _ := supplyCoordinator(t, t.TempDir())
	b.loadGraph = coreRouterGraph
	oc := enforceOrchestrator
	oc.Admission = "enforce"
	running, candidate := coreAndRouterTodos()

	admit := b.cellAdmission(oc, 1, nil, nil)

	if got := admit([]fleet.Todo{running}, []fleet.Todo{candidate}); got != 0 {
		t.Fatalf("admit = %d, want 0: the new rule admits router beside core", got)
	}
	if got := admit([]fleet.Todo{running}, []fleet.Todo{{ID: "core-two", Files: []string{"go/internal/core/other.go"}}}); got != -1 {
		t.Errorf("admit of a second core todo = %d, want -1", got)
	}
	if events := supplyEvents(t, b, evolveDir, orchestrator.CodeAdmissionShadow); len(events) != 0 {
		t.Errorf("shadow events = %+v, want none at enforce", events)
	}
}

func TestOrchestratorPool_ThePoolAdmitReachesTheAdmissionShadow(t *testing.T) {
	t.Parallel()
	b, evolveDir, _ := supplyCoordinator(t, t.TempDir())
	b.loadGraph = coreRouterGraph
	fc := policy.FleetConfig{Count: 2, PlanSource: "triage", Orchestrator: enforceOrchestrator}
	running, candidate := coreAndRouterTodos()

	p := b.orchestratorPool(fc, nil, 1)

	if p.Admit == nil {
		t.Fatal("pool admit = nil, want admission by cell")
	}
	if got := p.Admit([]fleet.Todo{running}, []fleet.Todo{candidate}); got != -1 {
		t.Fatalf("pool admit = %d, want -1 at shadow", got)
	}
	if events := supplyEvents(t, b, evolveDir, orchestrator.CodeAdmissionShadow); len(events) != 1 {
		t.Errorf("shadow events = %+v, want one from the production pool", events)
	}
}

func TestOrchestratorPool_AtEnforceTheAdmitRecordsTheWouldBeHolderThePoolCarries(t *testing.T) {
	t.Parallel()
	b, _, _ := supplyCoordinator(t, t.TempDir())
	b.loadGraph = coreRouterGraph
	oc := enforceOrchestrator
	oc.Admission = "enforce"
	running, candidate := coreAndRouterTodos()

	p := b.orchestratorPool(policy.FleetConfig{Count: 2, PlanSource: "triage", Orchestrator: oc}, nil, 1)
	if p.Holders == nil {
		t.Fatal("pool holders = nil, want the reach holders the admission records into")
	}
	if got := p.Admit([]fleet.Todo{running}, []fleet.Todo{candidate}); got != 0 {
		t.Fatalf("admit = %d, want 0 at enforce", got)
	}
	var env map[string]string
	carry := orchestrator.Pool{Target: 1, Holders: p.Holders, Launch: func(_ context.Context, s fleet.CycleSpec) (int, error) {
		env = s.Env
		return 0, nil
	}}
	carry.Run(context.Background(), []fleet.Todo{candidate})
	if env[ipcenv.FleetReachHolderKey] != "core-lane" {
		t.Errorf("launch env = %v, want %s=core-lane from the production admission", env, ipcenv.FleetReachHolderKey)
	}
}

package main

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func wiredRealPhasesWith(t *testing.T, files map[string]string) orchDeps {
	t.Helper()
	root := realPhasesRoot(t)
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	d := wireOrchestratorDeps(root, filepath.Join(root, ".evolve"), io.Discard, routingRun{})
	if d.RoutingErr != nil {
		t.Fatalf("wireOrchestratorDeps: %v", d.RoutingErr)
	}
	return d
}

func TestWireOrchestratorDeps_EveryPhaseRunnerJudgesWithTheContractGateReviewer(t *testing.T) {
	d := wiredWithRealPhases(t)
	gate := d.Orchestrator.ContractGateReviewer()
	if gate == nil {
		t.Fatal("contract_gate defaults to enforce, so the reviewer chain holds the contract gate")
	}
	for phase, r := range d.Runners {
		if phase == core.PhaseShip {
			continue
		}
		probe, ok := r.(interface{ ContractVerifierIs(any) bool })
		if !ok {
			t.Errorf("phase %s: runner %T exposes no ContractVerifierIs", phase, r)
			continue
		}
		if !probe.ContractVerifierIs(gate) {
			t.Errorf("phase %s: the verdict engine does not judge with the contract gate's Reviewer %T", phase, gate)
		}
	}
}

func TestWireOrchestratorDeps_DynamicLLMRoutingProposesThroughTheAdvisor(t *testing.T) {
	d := wiredWithRealPhases(t)
	if mode := d.Orchestrator.RoutingConfig().Mode; mode != config.ModeDynamicLLM {
		t.Fatalf("the default routing mode is dynamic_llm, got %v", mode)
	}
	proposal, ok := d.Orchestrator.RoutingStrategy().(router.LLMProposal)
	if !ok {
		t.Fatalf("dynamic_llm with a wired advisor routes through LLMProposal, got %#v", d.Orchestrator.RoutingStrategy())
	}
	if advisor, isAdvisor := proposal.Proposer.(*core.PhaseAdvisor); !isAdvisor || advisor == nil {
		t.Fatalf("the proposer is the wired routing advisor, not a nil one: %#v", proposal.Proposer)
	}
}

func TestWireOrchestratorDeps_APolicyShipFloorOverrideReachesTheOrchestrator(t *testing.T) {
	d := wiredRealPhasesWith(t, map[string]string{".evolve/policy.json": `{"ship_floor":["build","audit"]}`})
	want, _ := policy.Policy{ShipFloor: []string{"build", "audit"}}.FloorPhases()
	if got := d.Orchestrator.ShipFloor(); !slices.Equal(got, want) || slices.Equal(got, router.DefaultShipFloor()) {
		t.Fatalf("ShipFloor() = %v, want the policy override %v, not the default %v", got, want, router.DefaultShipFloor())
	}
}

func TestWireOrchestratorDeps_AFleetLaneShipsWithTheIntegrationStage(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	d := wiredRealPhasesWith(t, map[string]string{".evolve/policy.json": `{"fleet":{"integration":{"stage":"shadow"}}}`})
	probe, ok := d.Runners[core.PhaseShip].(interface{ IntegrationWired() bool })
	if !ok || !probe.IntegrationWired() {
		t.Fatalf("a fleet lane ship phase carries the integration stage, got %T", d.Runners[core.PhaseShip])
	}
}

func TestWireOrchestratorDeps_AUserPhaseAnchorChangesTheRoutedPlan(t *testing.T) {
	plain := wiredWithRealPhases(t)
	if slices.Contains(plain.Orchestrator.RoutingConfig().Order, "release-notes") {
		t.Fatal("the built-in order holds no release-notes phase, so the anchor is the only route to it")
	}
	d := wiredRealPhasesWith(t, map[string]string{".evolve/phases/release-notes/phase.json": `{"name":"release-notes","archetype":"learn","agent":"evolve-memo","outputs":{"files":[".evolve/runs/cycle-{cycle}/release-notes.md"]},"routing":{"insert_when":[{"field":"ship.class","op":"==","value":"cycle"}]},"after":"ship","optional":true}`})
	order := d.Orchestrator.RoutingConfig().Order
	ship := slices.Index(order, "ship")
	if ship < 0 || slices.Index(order, "release-notes") != ship+1 {
		t.Fatalf("the user anchor after:ship splices release-notes after ship in the routed order, got %v", order)
	}
	if _, ok := d.Orchestrator.RoutingConfig().Triggers["release-notes"]; !ok {
		t.Fatal("the user routing block becomes the release-notes trigger")
	}
}

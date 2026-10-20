package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func TestOrchestratorPool_TheDeadlineSealOfAStoppedLaneLandsInItsCycleStream(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	b, _ := iscsCoordinator(root, paths.EvolveDirOf(root), nil)
	ws := paths.RunWorkspace(root, 1955)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "lane-scope.json"), []byte(`{"todo_ids":["slow"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p := b.orchestratorPool(policy.FleetConfig{Count: 1, PlanSource: "triage", Orchestrator: enforceOrchestrator}, nil, 1)

	cycle, ok := p.Locate(orchestrator.Lane{Key: "slow", Scope: []string{"slow"}, Started: time.Now().Add(-time.Hour)})
	_, other := p.Locate(orchestrator.Lane{Key: "fast", Scope: []string{"fast"}, Started: time.Now().Add(-time.Hour)})
	if cycle != 1955 || !ok || other {
		t.Fatalf("locate = %d %t, other lane %t; want cycle 1955 for slow and nothing for fast", cycle, ok, other)
	}
	p.Publish(signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleLoop, Kind: signalcenter.KindCycleSealed, Severity: signalcenter.SeverityInfo,
		Fields: map[string]string{cyclestate.SealedTerminationField: cyclestate.CycleTerminationLaneDeadline}})
	b.deps.Signals.Flush()

	cycles, _ := wave.ReadCycles(root, 1954, 0)
	if len(cycles) != 1 || cycles[0].Termination != cyclestate.CycleTerminationLaneDeadline || cycles[0].EndedAt.IsZero() {
		t.Errorf("cycles = %+v, want cycle 1955 ended with the lane deadline termination from the loop's own stream writer", cycles)
	}
}

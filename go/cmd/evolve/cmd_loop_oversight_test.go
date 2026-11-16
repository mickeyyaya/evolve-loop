package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const oversightHandledTimeout = 5 * time.Second

type scriptedOversightSource struct {
	batches    chan []reader.Item
	handled    chan struct{}
	started    atomic.Bool
	ctxEnded   atomic.Bool
	afterBatch bool
}

func newScriptedOversightSource() *scriptedOversightSource {
	return &scriptedOversightSource{batches: make(chan []reader.Item, 4), handled: make(chan struct{}, 4)}
}

func (s *scriptedOversightSource) Next(ctx context.Context, _ time.Time) ([]reader.Item, error) {
	s.started.Store(true)
	if s.afterBatch {
		s.afterBatch = false
		s.handled <- struct{}{}
	}
	select {
	case items := <-s.batches:
		s.afterBatch = true
		return items, nil
	case <-ctx.Done():
		s.ctxEnded.Store(true)
		return nil, ctx.Err()
	}
}

func (s *scriptedOversightSource) deliverAndAwaitHandled(t *testing.T, e signalcenter.Event) {
	t.Helper()
	s.batches <- []reader.Item{{Channel: "ship", Record: channel.Record{Source: "ship", Signal: &e}}}
	select {
	case <-s.handled:
	case <-time.After(oversightHandledTimeout):
		t.Errorf("the oversight never handled %s: nothing in the loop reads the event source", e.Kind)
	}
}

func oversightCoordinator(t *testing.T, src *scriptedOversightSource) (*loopBatchCoordinator, string, *lockedBuffer) {
	t.Helper()
	b, evolveDir, console := supplyCoordinator(t, t.TempDir())
	b.oversightSource = func() (orchestrator.Source, error) { return src, nil }
	return b, evolveDir, console
}

func oversightFleet(stage string) policy.FleetConfig {
	oc := enforceOrchestrator
	oc.Stage = stage
	oc.WindowStart, oc.WindowMin = 2, 1
	return policy.FleetConfig{Count: 4, Concurrency: 4, PlanSource: "triage", Orchestrator: oc}
}

func pinLaneCycle(t *testing.T, root string, cycle int, todo string) {
	t.Helper()
	dir := paths.RunWorkspace(root, cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(committedset.LanePinDoc{TodoIDs: []string{todo}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(committedset.LanePinPath(dir), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func landedLaunch(t *testing.T, root string, cycle int, src *scriptedOversightSource) fleet.LaunchFn {
	return func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		pinLaneCycle(t, root, cycle, spec.Scope[0])
		src.deliverAndAwaitHandled(t, signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleShip, Kind: signalcenter.KindShipLanded, Severity: signalcenter.SeverityInfo})
		return 0, nil
	}
}

func TestOrchestratedRun_ConstructsAndRunsOversightAndStopsItWithThePool(t *testing.T) {
	t.Parallel()
	src := newScriptedOversightSource()
	b, _, _ := oversightCoordinator(t, src)
	var startedDuringRun bool
	launch := func(_ context.Context, _ fleet.CycleSpec) (int, error) {
		src.deliverAndAwaitHandled(t, signalcenter.Event{Module: signalcenter.ModuleLoop, Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo})
		startedDuringRun = src.started.Load()
		return 0, nil
	}
	res := b.orchestratedRun(oversightFleet("enforce"), launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}}, noResupply)
	if len(res) != 1 || res[0].ExitCode != 0 {
		t.Fatalf("results = %+v, want the one lane to exit 0", res)
	}
	if !startedDuringRun {
		t.Error("the oversight never read its event source while the pool ran")
	}
	if !src.ctxEnded.Load() {
		t.Error("the oversight was still running when the pool returned: its context was not cancelled and waited for")
	}
}

func TestOrchestratedRun_ACleanLandingMovesTheAdaptiveWindowAtEnforce(t *testing.T) {
	t.Parallel()
	src := newScriptedOversightSource()
	b, evolveDir, _ := oversightCoordinator(t, src)
	launch := landedLaunch(t, b.cfg.ProjectRoot, 7, src)
	b.orchestratedRun(oversightFleet("enforce"), launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}}, noResupply)
	moves := supplyEvents(t, b, evolveDir, orchestrator.CodeWindow)
	if len(moves) != 1 {
		t.Fatalf("window events = %+v, want one move from the clean landing", moves)
	}
	got := moves[0].Fields
	if got["old"] != "2" || got["new"] != "3" || got["cause"] != "clean_landing" {
		t.Errorf("window move fields = %v, want old 2 to new 3 by clean_landing", got)
	}
	actions := supplyEvents(t, b, evolveDir, orchestrator.CodeOversightAction)
	if len(actions) != 1 || actions[0].Fields["applied"] != "true" {
		t.Errorf("oversight actions = %+v, want one applied action at enforce", actions)
	}
}

func TestOrchestratedRun_ALandingAtShadowIsRecordedAndMovesNothing(t *testing.T) {
	t.Parallel()
	src := newScriptedOversightSource()
	b, evolveDir, _ := oversightCoordinator(t, src)
	launch := landedLaunch(t, b.cfg.ProjectRoot, 7, src)
	b.orchestratedRun(oversightFleet("shadow"), launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}}, noResupply)
	if moves := supplyEvents(t, b, evolveDir, orchestrator.CodeWindow); len(moves) != 0 {
		t.Errorf("window events = %+v, want none at shadow", moves)
	}
	actions := supplyEvents(t, b, evolveDir, orchestrator.CodeOversightAction)
	if len(actions) != 1 || actions[0].Fields["applied"] != "false" {
		t.Errorf("oversight actions = %+v, want one unapplied action at shadow", actions)
	}
}

func TestOrchestratedRun_AnEventSourceThatCannotOpenLeavesThePoolRunningWithAWarning(t *testing.T) {
	t.Parallel()
	b, _, console := supplyCoordinator(t, t.TempDir())
	b.oversightSource = func() (orchestrator.Source, error) { return nil, errors.New("kernel wake refused") }
	var launched []string
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		launched = append(launched, spec.Scope[0])
		return 0, nil
	}
	res := b.orchestratedRun(oversightFleet("enforce"), launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}}, noResupply)
	if len(res) != 1 || res[0].ExitCode != 0 || res[0].Err != nil || len(launched) != 1 {
		t.Errorf("results = %+v launched = %v, want the pool to run its lane without the oversight", res, launched)
	}
	if !strings.Contains(console.String(), "[loop] WARN: orchestrator: oversight") || !strings.Contains(console.String(), "kernel wake refused") {
		t.Errorf("console = %q, want a WARN that names the oversight and the refusal", console.String())
	}
}

func TestOrchestratedRun_TheOversightHaltReachesThePoolIntakeWhenCellAdmissionLoads(t *testing.T) {
	t.Parallel()
	src := newScriptedOversightSource()
	b, _, _ := oversightCoordinator(t, src)
	b.loadGraph = func(context.Context, string, int) (*admission.ModuleGraph, error) {
		return admission.NewModuleGraph(admission.ModulePath+"/internal/x|\n"+admission.ModulePath+"/internal/y|\n", 120)
	}
	fc := oversightFleet("enforce")
	fc.Count, fc.Concurrency = 2, 1
	var launched []string
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		launched = append(launched, spec.Scope[0])
		src.deliverAndAwaitHandled(t, signalcenter.Event{Module: signalcenter.ModuleLoop, Kind: signalcenter.KindLoopHalt, Severity: signalcenter.SeverityIncident})
		return 0, nil
	}
	backlog := []fleet.Todo{{ID: "x1", Files: []string{"go/internal/x/x.go"}}, {ID: "y1", Files: []string{"go/internal/y/y.go"}}}
	res := b.orchestratedRun(fc, launch, 2)(context.Background(), backlog, noResupply)
	if len(launched) != 1 || launched[0] != "x1" {
		t.Errorf("launched = %v (results %d), want only x1: the loop.halt event must stop admission of y1 through the pool's own intake", launched, len(res))
	}
}

func noResupply([]fleet.Todo) ([]fleet.Todo, error) { return nil, nil }

func TestOrchestratedRun_AWaveStopThatMeetsTheRunCapNamesTheOperatorStop(t *testing.T) {
	t.Parallel()
	src := newScriptedOversightSource()
	b, evolveDir, _ := oversightCoordinator(t, src)
	fc := oversightFleet("enforce")
	fc.Orchestrator.RunMaxLanes = 1
	launch := func(_ context.Context, _ fleet.CycleSpec) (int, error) {
		if err := os.WriteFile(paths.LoopStopPath(evolveDir), nil, 0o644); err != nil {
			t.Error(err)
		}
		return 0, nil
	}
	b.orchestratedRun(fc, launch, 2)(context.Background(), []fleet.Todo{{ID: "first"}, {ID: "second"}}, noResupply)
	idle := supplyEvents(t, b, evolveDir, orchestrator.CodePoolIdle)
	if len(idle) == 0 {
		t.Fatalf("%s events = 0, want the idle transition of the drained pool", orchestrator.CodePoolIdle)
	}
	for _, e := range idle {
		if e.Fields["cause"] != string(orchestrator.CauseOperatorStop) {
			t.Errorf("idle cause = %q, want %q: the wave stop drained a pool that was also at run_max_lanes", e.Fields["cause"], orchestrator.CauseOperatorStop)
		}
	}
	if n := len(supplyEvents(t, b, evolveDir, orchestrator.CodePoolUnderfilled)); n != 0 {
		t.Errorf("%s events = %d, want 0 after an operator stop", orchestrator.CodePoolUnderfilled, n)
	}
}

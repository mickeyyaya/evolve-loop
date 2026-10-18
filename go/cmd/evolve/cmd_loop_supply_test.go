package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const supplyModuleGraph = "github.com/mickeyyaya/evolve-loop/go/internal/alpha|\ngithub.com/mickeyyaya/evolve-loop/go/internal/beta|"

type planProbe struct {
	fixtures.FakeStorage
	recordPath string
	mu         sync.Mutex
	onDisk     []bool
}

func (p *planProbe) ReadState(ctx context.Context) (core.State, error) {
	_, err := os.Stat(p.recordPath)
	p.mu.Lock()
	p.onDisk = append(p.onDisk, err == nil)
	p.mu.Unlock()
	return p.FakeStorage.ReadState(ctx)
}

func (p *planProbe) reads() []bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]bool(nil), p.onDisk...)
}

func seedSupplyInbox(t *testing.T, evolveDir string) {
	t.Helper()
	inbox := filepath.Join(evolveDir, "inbox")
	u13WriteJSON(t, filepath.Join(inbox, "lane-a.json"), map[string]any{"id": "lane-a", "weight": 0.9, "priority_class": "feature", "files": []string{"go/internal/alpha/a.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "lane-b.json"), map[string]any{"id": "lane-b", "weight": 0.8, "priority_class": "feature", "files": []string{"go/internal/alpha/b.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "console-x.json"), map[string]any{"id": "console-x", "weight": 0.7, "priority_class": "feature", "route": "console", "files": []string{"go/internal/beta/x.go"}})
}

func supplyCoordinator(t *testing.T, root string) (*loopBatchCoordinator, string, *lockedBuffer) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	console := &lockedBuffer{}
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: console}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, console)
	b.deps.Storage = &fixtures.FakeStorage{}
	b.loadGraph = noModuleGraph
	return b, evolveDir, console
}

func supplyEvents(t *testing.T, b *loopBatchCoordinator, evolveDir string, code signalcenter.Code) []signalcenter.Event {
	t.Helper()
	b.deps.Signals.Flush()
	chunk, err := signalcenter.ReadStream(filepath.Join(evolveDir, signalcenter.StreamFileName), 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []signalcenter.Event
	for _, e := range chunk.Events {
		if e.Code == code {
			out = append(out, e)
		}
	}
	return out
}

func noModuleGraph(context.Context, string, int) (*admission.ModuleGraph, error) {
	return nil, errors.New("the test loads no module graph")
}

func constantGraph(loads *int) func(context.Context, string, int) (*admission.ModuleGraph, error) {
	return func(context.Context, string, int) (*admission.ModuleGraph, error) {
		*loads++
		return admission.NewModuleGraph(supplyModuleGraph, 1000)
	}
}

func TestDispatchFleetIteration_TakesTheSupplyBeforeThePlan(t *testing.T) {
	t.Parallel()
	for name, fc := range map[string]policy.FleetConfig{
		"the barrier wave":      {Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave"},
		"the rolling pool":      {Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"},
		"the orchestrator pool": {Count: 2, Concurrency: 2, PlanSource: "triage", Orchestrator: enforceOrchestrator},
	} {
		t.Run(name, func(t *testing.T) {
			b, evolveDir, _ := supplyCoordinator(t, gittest.Fixture(t).Dir)
			seedSupplyInbox(t, evolveDir)
			probe := &planProbe{recordPath: filepath.Join(evolveDir, "waves", orchestrator.SupplyFile)}
			b.deps.Storage = probe
			var starvation fleet.StarvationTracker

			d := b.dispatchFleetIteration(0, fc, "/nonexistent/evolve", &starvation)

			got := supplyEvents(t, b, evolveDir, orchestrator.CodeSupply)
			if len(got) != 1 || got[0].Fields[orchestrator.ClassAdmissible] != "2" || got[0].Fields[orchestrator.ClassConsoleRoute] != "1" {
				t.Errorf("supply events = %+v, want one that counts 2 admissible items and 1 console route item", got)
			}
			history, err := orchestrator.ReadSupply(probe.recordPath)
			if err != nil || len(history) != 1 || history[0].Counts[orchestrator.ClassAdmissible] != 2 || len(history[0].Refusals) != 1 {
				t.Errorf("record = %+v, %v, want the one census of this plan with its one refusal", history, err)
			}
			reads := probe.reads()
			if len(reads) == 0 {
				t.Fatal("the plan never read the state, so the test proved nothing about its order")
			}
			for i, onDisk := range reads {
				if !onDisk {
					t.Errorf("read %d of the state ran before the supply record was on disk, want the census before the plan", i)
				}
			}
			if d.flow == batchReturn {
				t.Errorf("decision = %+v, want the batch to go on: the census never halts it", d)
			}
		})
	}
}

func TestDispatchFleetIteration_TakesNoSupplyWhenNoPlanWillRun(t *testing.T) {
	t.Parallel()
	for name, fc := range map[string]policy.FleetConfig{
		"one lane":              {Count: 1, Concurrency: 1, PlanSource: "triage"},
		"a manual plan source":  {Count: 2, Concurrency: 2, PlanSource: "manual"},
		"the empty fleet value": {},
	} {
		t.Run(name, func(t *testing.T) {
			b, evolveDir, _ := supplyCoordinator(t, t.TempDir())
			seedSupplyInbox(t, evolveDir)
			var starvation fleet.StarvationTracker

			b.dispatchFleetIteration(0, fc, "/nonexistent/evolve", &starvation)

			if got := supplyEvents(t, b, evolveDir, orchestrator.CodeSupply); len(got) != 0 {
				t.Errorf("supply events = %+v, want none: no plan runs", got)
			}
			if _, err := os.Stat(filepath.Join(evolveDir, "waves", orchestrator.SupplyFile)); err == nil {
				t.Error("a supply record was written, want none")
			}
		})
	}
}

func TestDispatchFleetIteration_ASupplyErrorWarnsAndNeverFailsTheIteration(t *testing.T) {
	t.Parallel()
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave"}
	for name, tc := range map[string]struct {
		sabotage   func(t *testing.T, evolveDir string)
		wantCause  string
		wantEvents int
	}{
		"an inbox that cannot be read": {
			sabotage: func(t *testing.T, evolveDir string) {
				if err := os.RemoveAll(filepath.Join(evolveDir, "inbox")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(evolveDir, "inbox"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantCause: "read dir", wantEvents: 0,
		},
		"a record that cannot be written": {
			sabotage: func(t *testing.T, evolveDir string) {
				if err := os.WriteFile(filepath.Join(evolveDir, "waves"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantCause: "supply record", wantEvents: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			b, evolveDir, console := supplyCoordinator(t, t.TempDir())
			seedSupplyInbox(t, evolveDir)
			tc.sabotage(t, evolveDir)
			var starvation fleet.StarvationTracker

			d := b.dispatchFleetIteration(0, fc, "/nonexistent/evolve", &starvation)

			if d.flow != batchProceed {
				t.Errorf("decision = %+v, want the iteration to go on: the census is advisory", d)
			}
			if !strings.Contains(console.String(), "[loop] WARN: orchestrator: census 0:") || !strings.Contains(console.String(), tc.wantCause) {
				t.Errorf("console = %q, want the census WARN with the cause %q", console.String(), tc.wantCause)
			}
			if got := supplyEvents(t, b, evolveDir, orchestrator.CodeSupply); len(got) != tc.wantEvents {
				t.Errorf("supply events = %d, want %d", len(got), tc.wantEvents)
			}
		})
	}
}

func TestTakeSupply_AndCellAdmissionShareOneModuleGraphLoadForEachIteration(t *testing.T) {
	t.Parallel()
	b, evolveDir, _ := supplyCoordinator(t, t.TempDir())
	seedSupplyInbox(t, evolveDir)
	loads := 0
	b.loadGraph = constantGraph(&loads)
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Orchestrator: enforceOrchestrator}

	b.takeSupply(3, fc)
	admit := b.cellAdmission(fc.Orchestrator, 3)
	if admit == nil || loads != 1 {
		t.Fatalf("admit set = %v, loads = %d, want one load shared by the census and the pool admission of iteration 3", admit != nil, loads)
	}
	b.takeSupply(4, fc)
	if loads != 2 {
		t.Errorf("loads after iteration 4 = %d, want 2: the next iteration reads the tree again", loads)
	}
	b.takeSupply(4, fc)
	b.cellAdmission(fc.Orchestrator, 4)
	if loads != 2 {
		t.Errorf("loads after a second census of iteration 4 = %d, want still 2", loads)
	}
}

func TestTakeSupply_WithTheModuleGraphTwoTodosOfOnePackageWaitForTheCellAndTheSupplyIsShort(t *testing.T) {
	t.Parallel()
	b, evolveDir, _ := supplyCoordinator(t, t.TempDir())
	seedSupplyInbox(t, evolveDir)
	loads := 0
	b.loadGraph = constantGraph(&loads)

	b.takeSupply(0, policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave"})

	history, err := orchestrator.ReadSupply(filepath.Join(evolveDir, "waves", orchestrator.SupplyFile))
	if err != nil || len(history) != 1 || history[0].Counts[orchestrator.ClassAdmissible] != 1 || history[0].Counts[orchestrator.ClassWaitingCell] != 1 {
		t.Fatalf("record = %+v, %v, want 1 admissible and 1 waiting for the alpha cell", history, err)
	}
	short := supplyEvents(t, b, evolveDir, orchestrator.CodeSupplyShort)
	if len(short) != 1 || short[0].Fields["window"] != "2" || short[0].Fields[orchestrator.ClassAdmissible] != "1" {
		t.Errorf("supply short events = %+v, want one: 1 admissible is below the window of 2", short)
	}
}

func TestTakeSupply_AModuleGraphThatDoesNotLoadCountsEveryReadyItemAndSaysWhy(t *testing.T) {
	t.Parallel()
	b, evolveDir, console := supplyCoordinator(t, t.TempDir())
	seedSupplyInbox(t, evolveDir)
	loads := 0
	b.loadGraph = func(context.Context, string, int) (*admission.ModuleGraph, error) {
		loads++
		return nil, errors.New("go list is gone")
	}
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Orchestrator: enforceOrchestrator}

	b.takeSupply(5, fc)
	admit := b.cellAdmission(fc.Orchestrator, 5)

	history, err := orchestrator.ReadSupply(filepath.Join(evolveDir, "waves", orchestrator.SupplyFile))
	if err != nil || len(history) != 1 || history[0].Counts[orchestrator.ClassAdmissible] != 2 || len(history[0].Warnings) != 1 {
		t.Fatalf("record = %+v, %v, want both ready items admissible and one warning in the record", history, err)
	}
	if admit != nil || loads != 1 {
		t.Errorf("admit set = %v, loads = %d, want the file rule after one failed load that both callers share", admit != nil, loads)
	}
	if !strings.Contains(console.String(), "census 5 counts without cells; the module graph did not load: go list is gone") {
		t.Errorf("console = %q, want the census WARN that names the cause", console.String())
	}
}

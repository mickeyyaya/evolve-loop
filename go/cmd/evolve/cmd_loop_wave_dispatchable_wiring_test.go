package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func wave20ShapedRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeWave20Shape(t, root)
	return root
}

func writeWave20Shape(t *testing.T, root string) {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	u13WriteJSON(t, filepath.Join(inbox, "consumed", "shipped-a.json"), map[string]any{"id": "shipped-a", "files": []string{"pkg/a.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "consumed", "shipped-b.json"), map[string]any{"id": "shipped-b", "files": []string{"pkg/b.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "console-dep.json"), map[string]any{"id": "console-dep", "weight": 0.9, "route": "console", "files": []string{"pkg/console.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "blocked-a.json"), map[string]any{"id": "blocked-a", "weight": 0.8, "deps": []string{"console-dep"}, "files": []string{"pkg/c.go"}})
	u13WriteJSON(t, filepath.Join(inbox, "blocked-b.json"), map[string]any{"id": "blocked-b", "weight": 0.7, "deps": []string{"console-dep"}, "files": []string{"pkg/d.go"}})
	if err := os.MkdirAll(cycleWorkspace(root, 7), 0o755); err != nil {
		t.Fatal(err)
	}
	decision := `{"top_n":[{"id":"shipped-a","files":["pkg/a.go"]},{"id":"shipped-b","files":["pkg/b.go"]}]}`
	if err := os.WriteFile(filepath.Join(cycleWorkspace(root, 7), "triage-decision.json"), []byte(decision), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWaveWithNoDispatchableLaneTakesTheEmptyBacklogPathInsteadOfACountedEmptyWave(t *testing.T) {
	root := wave20ShapedRoot(t)
	cfg := loopConfig{ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve")}
	fleetCfg := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "wave"}
	storage := &fixtures.FakeStorage{State: core.State{LastCycleNumber: 7}}
	plan := productionWavePlanFn(cfg, storage, fleetCfg.Count, io.Discard)
	var gate bytes.Buffer
	launcher := productionWaveLauncher(fleetCfg, "", root, "", "", io.Discard, &gate)
	ready := func() error { return nil }

	ran, specs, results, err := dispatchIteration(context.Background(), fleetCfg, ready, plan, launcher, nil, 0)
	if err != nil || ran || len(specs) != 0 || len(results) != 0 {
		t.Fatalf("no dispatchable lane, so the wave must not run: ran=%v specs=%v results=%v err=%v", ran, specs, results, err)
	}
	var stderr bytes.Buffer
	handled := minWidthRepair(context.Background(), fleetCfg, fleetCfg, ready, plan, launcher, nil, 0, &stderr, testRootSignals(t, &stderr))
	if handled || !strings.Contains(stderr.String(), "empty backlog") {
		t.Errorf("the one-lane repair finds nothing either and names the empty backlog: handled=%v %q", handled, stderr.String())
	}
	if strings.Contains(gate.String(), "freshness gate skipped") {
		t.Errorf("the plan offered a lane the launch gate had to refuse: %q", gate.String())
	}
}

func TestZeroLaneWavesCountTowardWorkSupplyStarvation(t *testing.T) {
	root := gittest.Fixture(t).Dir
	writeWave20Shape(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	var console bytes.Buffer
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: &console}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, &console)
	b.deps.Storage = &fixtures.FakeStorage{State: core.State{LastCycleNumber: 7}}
	fleetCfg := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave", StarvationK: 2, StarvationWeight: 0.9}
	var starvation fleet.StarvationTracker
	for wave := 0; wave < 2; wave++ {
		if d := b.dispatchFleetIteration(wave, fleetCfg, "/nonexistent/evolve", &starvation); d.flow != batchProceed {
			t.Fatalf("wave %d: a wave with nothing to launch falls through: %+v", wave, d)
		}
	}
	b.deps.Signals.Flush()
	out := console.String()
	if !strings.Contains(out, "LOOP_WAVE_EMPTY_PLAN") || !strings.Contains(out, "work-supply starvation after 2 waves") {
		t.Errorf("two zero-lane waves take the empty-plan path and are two starved waves:\n%s", out)
	}
}

func TestConsoleRoutedResolver_ReadsTheConfiguredEvolveDir(t *testing.T) {
	moved := filepath.Join(t.TempDir(), "elsewhere")
	u13WriteJSON(t, filepath.Join(moved, "inbox", "console-item.json"), map[string]any{"id": "console-item", "route": "console"})
	cfg := loopConfig{ProjectRoot: t.TempDir(), EvolveDir: moved}
	if routed, _ := consoleRoutedResolver(cfg, io.Discard)("console-item"); !routed {
		t.Error("the pool's routing backstop must read the same evolve dir as its plan")
	}
}

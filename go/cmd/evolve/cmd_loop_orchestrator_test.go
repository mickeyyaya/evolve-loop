package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var enforceOrchestrator = policy.OrchestratorConfig{Stage: "enforce", ScanK: 32, HubMinImporters: 120, DeadlineFloorMinutes: 60, StallMinutes: 20, RunMaxLanes: 8, WindowStart: 4, WindowMin: 2}

func writeLaneSpan(t *testing.T, root string, cycle int, span time.Duration) {
	t.Helper()
	start := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	var lines []string
	for i, at := range []time.Time{start, start.Add(span)} {
		b, _ := json.Marshal(signalcenter.Event{Seq: uint64(i + 1), TS: at.Format(time.RFC3339Nano), Module: signalcenter.ModuleLoop, Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo})
		lines = append(lines, string(b))
	}
	dir := paths.RunWorkspace(root, cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, signalcenter.StreamFileName), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewLaneLeases_TheDeadlineIsTheP95LaneTimeWithoutTheLongestLaneOverTheFloor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for i, span := range []time.Duration{90 * time.Minute, 100 * time.Minute, 120 * time.Minute} {
		writeLaneSpan(t, root, i+1, span)
	}
	var stderr bytes.Buffer
	_, cfg := newLaneLeases(root, enforceOrchestrator, nil, &stderr)
	want := orchestrator.LeaseConfig{Stage: "enforce", Deadline: 100 * time.Minute, Stall: 20 * time.Minute}
	if cfg != want {
		t.Errorf("lease config = %+v, want %+v", cfg, want)
	}
	if !strings.Contains(stderr.String(), "[loop] orchestrator: lane lease (stage enforce): deadline 1h40m0s from 3 lane time(s), stall 20m0s") {
		t.Errorf("stderr = %q, want the lease line", stderr.String())
	}
}

func TestNewLaneLeases_NoRunHistoryTakesTheFloorAndSaysSo(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	_, cfg := newLaneLeases(t.TempDir(), enforceOrchestrator, nil, &stderr)
	if cfg.Deadline != time.Hour {
		t.Errorf("deadline = %v, want the 60-minute floor", cfg.Deadline)
	}
	if !strings.Contains(stderr.String(), "[loop] WARN: orchestrator: no lane times") {
		t.Errorf("stderr = %q, want a WARN that names the missing history", stderr.String())
	}
}

func TestLaneLeases_TheCoordinatorKeepsOneLeaseTableForEachStage(t *testing.T) {
	t.Parallel()
	b, _ := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	shadow := enforceOrchestrator
	shadow.Stage = "shadow"
	first, again, enforce := b.laneLeases(shadow), b.laneLeases(shadow), b.laneLeases(enforceOrchestrator)
	if first != again {
		t.Error("two shadow calls gave two lease tables, want one: the stop count of a todo lives for the batch")
	}
	if first == enforce {
		t.Error("the enforce stage shares the shadow lease table, want its own")
	}
}

func TestWaveRequest_TheBarrierLaunchCarriesAShadowLease(t *testing.T) {
	t.Parallel()
	b, console := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Orchestrator: enforceOrchestrator}
	b.waveRequest(0, fc, "/nonexistent-binary")
	if _, ok := b.leases["shadow"]; !ok {
		t.Errorf("lease tables = %v, want a shadow table for the wave launch", b.leases)
	}
	if !strings.Contains(console.String(), "lane lease (stage shadow)") {
		t.Errorf("console = %q, want the shadow lease line", console.String())
	}
}

func TestPoolHalted_TheOperatorStopOrACancelledBatchEndsAdmission(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	b, _ := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	b.ctx = ctx
	if b.poolHalted() {
		t.Fatal("halted with no brake and a live batch")
	}
	if err := os.WriteFile(paths.LoopStopPath(b.cfg.EvolveDir), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !b.poolHalted() {
		t.Error("the operator brake did not halt admission")
	}
	if err := os.Remove(paths.LoopStopPath(b.cfg.EvolveDir)); err != nil {
		t.Fatal(err)
	}
	cancel()
	if !b.poolHalted() {
		t.Error("a cancelled batch did not halt admission")
	}
}

func TestOrchestratorPool_WiresTheEnforceKeysAndTheFileRuleWithoutAGraph(t *testing.T) {
	t.Parallel()
	b, console := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	fc := policy.FleetConfig{Count: 3, Concurrency: 2, PlanSource: "triage", Orchestrator: enforceOrchestrator}
	launch := func(context.Context, fleet.CycleSpec) (int, error) { return 0, nil }
	p := b.orchestratorPool(fc, launch, 4)
	if p.Target != 3 || p.Concurrency != 2 || p.MaxLanes != 8 || p.Leases != b.leases["enforce"] || p.Freshen == nil || p.Claims == nil || p.Reap == nil || p.Halted == nil || p.Publish == nil {
		t.Errorf("pool = %+v, want the fleet width, run_max_lanes and the enforce lease table", p)
	}
	if p.Stage != "enforce" || p.Window == nil || p.Window.Size() != 2 {
		t.Errorf("stage = %q window = %v, want the enforce stage and a window of 2: window.start 4 is capped by the concurrency cap 2, which is under the fleet count 3", p.Stage, p.Window)
	}
	if p.Admit != nil || !strings.Contains(console.String(), "[loop] WARN: orchestrator: pool 4 admits by the file rule") {
		t.Errorf("admit set = %v, console = %q, want the file rule with a WARN when the module graph does not load", p.Admit != nil, console.String())
	}
}

func TestOrchestratorPool_AdmitsByCellWhenTheModuleGraphLoads(t *testing.T) {
	t.Parallel()
	b, console := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	b.loadGraph = func(context.Context, string, int) (*admission.ModuleGraph, error) {
		return admission.NewModuleGraph(admission.ModulePath+"/internal/x|\n", 120)
	}
	p := b.orchestratorPool(policy.FleetConfig{Count: 2, PlanSource: "triage", Orchestrator: enforceOrchestrator}, nil, 1)
	if p.Admit == nil {
		t.Fatalf("admit = nil, console = %q, want admission by cell", console.String())
	}
	x := fleet.Todo{ID: "x1", Files: []string{"go/internal/x/x.go"}}
	if got := p.Admit([]fleet.Todo{x}, []fleet.Todo{{ID: "x2", Files: []string{"go/internal/x/y.go"}}}); got != -1 {
		t.Errorf("admit of a second todo in a busy cell = %d, want -1", got)
	}
	if got := p.Admit(nil, []fleet.Todo{x}); got != 0 {
		t.Errorf("admit of a todo in a free cell = %d, want 0", got)
	}
}

func TestLaneWindow_OpensAtWindowStartPublishesEachChangeAndSaysSoOnTheConsole(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	b, console := iscsCoordinator(t.TempDir(), evolveDir, nil)
	fc := policy.FleetConfig{Count: 8, Concurrency: 8, PlanSource: "triage", Orchestrator: enforceOrchestrator}

	w := b.laneWindow(fc)
	if w.Size() != 4 {
		t.Fatalf("window = %d, want window.start 4 of the fleet count 8", w.Size())
	}
	if !strings.Contains(console.String(), "[loop] orchestrator: adaptive window (stage enforce): 4 of 8 lane(s), min 2") {
		t.Errorf("console = %q, want the line that names the opening window", console.String())
	}
	w.CleanLanding()
	b.deps.Signals.Flush()
	raw, err := os.ReadFile(filepath.Join(evolveDir, "signals.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"code":"LOOP_ORCH_WINDOW"`) || !strings.Contains(string(raw), `"cause":"clean_landing"`) {
		t.Errorf("signals = %s, want LOOP_ORCH_WINDOW with the cause clean_landing", raw)
	}
}

func TestLaneWindow_ARunStartsAtWindowStartAndTheLoopKeepsNoWindowBetweenRuns(t *testing.T) {
	t.Parallel()
	b, _ := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
	fc := policy.FleetConfig{Count: 8, Concurrency: 8, PlanSource: "triage", Orchestrator: enforceOrchestrator}
	first := b.laneWindow(fc)
	for range 4 {
		first.CleanLanding()
	}
	second := b.laneWindow(fc)
	if second == first || second.Size() != 4 {
		t.Errorf("second window = %p size %d, want a new window at window.start 4: a run is one boundary-to-boundary unit", second, second.Size())
	}
}

func TestLaneWindow_TheSizedCountIsTheSmallerOfTheFleetCountAndTheConcurrencyCap(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		count, concurrency       int
		wantSize, afterAConflict int
		wantLine                 string
	}{
		"a cap under the count": {16, 6, 6, 3, "6 of 6 lane(s), min 2"},
		"a cap over the count":  {8, 10, 8, 4, "8 of 8 lane(s), min 2"},
		"no cap":                {8, 0, 8, 4, "8 of 8 lane(s), min 2"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, console := iscsCoordinator(t.TempDir(), t.TempDir(), nil)
			oc := enforceOrchestrator
			oc.WindowStart = 12
			w := b.laneWindow(policy.FleetConfig{Count: tc.count, Concurrency: tc.concurrency, PlanSource: "triage", Orchestrator: oc})
			if w.Size() != tc.wantSize {
				t.Fatalf("window = %d, want %d: window.start 12 is capped by the pool width", w.Size(), tc.wantSize)
			}
			w.Conflict()
			if w.Size() != tc.afterAConflict {
				t.Errorf("window after a conflict = %d, want %d: a halving binds at once only when the window never exceeds the pool width", w.Size(), tc.afterAConflict)
			}
			if !strings.Contains(console.String(), tc.wantLine) {
				t.Errorf("console = %q, want the line to end with %q", console.String(), tc.wantLine)
			}
		})
	}
}

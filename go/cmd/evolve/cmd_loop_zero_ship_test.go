package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const (
	zsShipped = `[{"phase":"ship","verdict":"PASS"}]`
	zsNoWork  = `[{"phase":"triage","verdict":"SKIPPED","abort_reason":"triage-empty-commitment"}]`
	zsFailed  = `[{"phase":"build","verdict":"FAIL","abort_reason":"build-contract-gate-block"}]`

	zsHaltCode  = "LOOP_ZERO_SHIP_HALT"
	zsFirstWave = 11
)

type zsClock struct{ cur time.Time }

func (c *zsClock) now() time.Time { return c.cur.Add(time.Second) }

type zsOrch struct {
	noSignals
	evolveDir string
	timings   []string
	calls     int
	clock     *zsClock
	t         *testing.T
}

func (o *zsOrch) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	timing := o.timings[min(o.calls, len(o.timings)-1)]
	o.calls++
	cycle := zsFirstWave - 1 + o.calls
	dir := filepath.Join(o.evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return core.CycleResult{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "phase-timing.json"), []byte(timing), 0o644); err != nil {
		return core.CycleResult{}, err
	}
	o.clock.cur = o.clock.cur.Add(time.Minute)
	if timing == zsShipped {
		zwShipAt(o.t, o.evolveDir, cycle, o.clock.cur)
	}
	return core.CycleResult{Cycle: cycle, FinalVerdict: core.VerdictPASS}, nil
}

func (o *zsOrch) RunCycleFromPhase(ctx context.Context, req core.CycleRequest, _ *core.ResumePoint) (core.CycleResult, error) {
	return o.RunCycle(ctx, req)
}

type zsRun struct {
	rc        int
	stderr    string
	calls     int
	inboxDir  string
	evolveDir string
}

func runZeroShipLoop(t *testing.T, failurePolicy string, cycles int, timings ...string) zsRun {
	t.Helper()
	return runZeroShipLoopSeeded(t, nil, failurePolicy, cycles, timings...)
}

func runZeroShipLoopSeeded(t *testing.T, seed func(evolveDir string), failurePolicy string, cycles int, timings ...string) zsRun {
	t.Helper()
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)
	policyJSON := `{"dispatch":{"policy":"off"},"workflow":{"max_consecutive_fails":9}` + failurePolicy + `}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if seed != nil {
		seed(evolveDir)
	}
	defer installStubDeps(t, &fixtures.FakeStorage{}, newFakeLedger())()
	prev, prevClock := loopOrchOverride, loopClock
	clock := &zsClock{cur: zwBase}
	orch := &zsOrch{evolveDir: evolveDir, timings: timings, clock: clock, t: t}
	loopOrchOverride, loopClock = orch, clock.now
	defer func() { loopOrchOverride, loopClock = prev, prevClock }()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "zero-ship goal",
		"--cycles", strconv.Itoa(cycles),
	}, nil, &stdout, &stderr)
	return zsRun{rc: rc, stderr: stderr.String(), calls: orch.calls, inboxDir: filepath.Join(evolveDir, "inbox"), evolveDir: evolveDir}
}

func zsInboxHasPipelineDefect(t *testing.T, inboxDir string) bool {
	t.Helper()
	entries, _ := os.ReadDir(inboxDir)
	for _, e := range entries {
		if strings.Contains(e.Name(), "pipeline-defect-zero-ship") {
			return true
		}
	}
	return false
}

func TestRunLoop_ZeroShip_TwoConsecutiveZeroShipWavesHaltBeforeTheThird(t *testing.T) {
	r := runZeroShipLoop(t, ``, 5, zsNoWork, zsNoWork)

	if r.calls != 2 {
		t.Fatalf("two consecutive zero-ship waves must halt before a third launches: launched %d of 5\n%s", r.calls, r.stderr)
	}
	if r.rc != systemFailureHaltExitCode {
		t.Errorf("the halt exits with the ADR-0072 system-failure code %d, got %d", systemFailureHaltExitCode, r.rc)
	}
	if !strings.Contains(r.stderr, zsHaltCode) {
		t.Errorf("the halt carries the zero-ship Signal Center code %s:\n%s", zsHaltCode, r.stderr)
	}
	if !zsInboxHasPipelineDefect(t, r.inboxDir) {
		t.Errorf("the halt files its P0 pipeline-defect item for category zero-ship in %s", r.inboxDir)
	}
}

func TestRunLoop_ZeroShip_FailedAndSkippedNoWorkWavesCountAsZeroShip(t *testing.T) {
	cases := map[string][]string{
		"a failed wave then a no-work wave": {zsFailed, zsNoWork},
		"a no-work wave then a failed wave": {zsNoWork, zsFailed},
		"two failed waves":                  {zsFailed, zsFailed},
	}
	for name, timings := range cases {
		timings := timings
		t.Run(name, func(t *testing.T) {
			r := runZeroShipLoop(t, ``, 5, timings...)
			if r.calls != 2 || r.rc != systemFailureHaltExitCode || !strings.Contains(r.stderr, zsHaltCode) {
				t.Fatalf("launched=%d rc=%d, want 2 launches then halt rc=%d with %s\n%s", r.calls, r.rc, systemFailureHaltExitCode, zsHaltCode, r.stderr)
			}
		})
	}
}

func TestRunLoop_ZeroShip_AShipResetsTheCount(t *testing.T) {
	t.Run("a ship between zero-ship waves never reaches the ceiling", func(t *testing.T) {
		r := runZeroShipLoop(t, ``, 5, zsNoWork, zsShipped, zsNoWork, zsShipped, zsNoWork)
		if r.calls != 5 || r.rc != 0 || strings.Contains(r.stderr, zsHaltCode) {
			t.Fatalf("launched=%d rc=%d, want all 5 waves and no zero-ship halt\n%s", r.calls, r.rc, r.stderr)
		}
		if zsInboxHasPipelineDefect(t, r.inboxDir) {
			t.Error("no halt, so no pipeline-defect item")
		}
	})
	t.Run("the count restarts after the ship", func(t *testing.T) {
		r := runZeroShipLoop(t, ``, 6, zsNoWork, zsShipped, zsNoWork, zsNoWork)
		if r.calls != 4 || r.rc != systemFailureHaltExitCode || !strings.Contains(r.stderr, zsHaltCode) {
			t.Fatalf("launched=%d rc=%d, want the halt after the second zero-ship wave following the ship (4 launches)\n%s", r.calls, r.rc, r.stderr)
		}
	})
}

func TestRunLoop_ZeroShip_ThresholdComesFromPolicyJSON(t *testing.T) {
	t.Run("a ceiling of three lets two zero-ship waves run and halts before the fourth", func(t *testing.T) {
		r := runZeroShipLoop(t, `,"failure_policy":{"thresholds":{"zero_ship_halt":3}}`, 6, zsNoWork)
		if r.calls != 3 || r.rc != systemFailureHaltExitCode || !strings.Contains(r.stderr, zsHaltCode) {
			t.Fatalf("launched=%d rc=%d, want 3 launches then halt\n%s", r.calls, r.rc, r.stderr)
		}
	})
	t.Run("the compiled default of two applies when policy.json sets nothing", func(t *testing.T) {
		r := runZeroShipLoop(t, `,"failure_policy":{"thresholds":{"lane_deferral_halt_ceiling":5}}`, 6, zsNoWork)
		if r.calls != 2 {
			t.Fatalf("launched=%d, want the default ceiling of 2\n%s", r.calls, r.stderr)
		}
	})
}

func zsCoordinator(t *testing.T, evolveDir, root string) *loopBatchCoordinator {
	t.Helper()
	u13StubRefresh(t, func() bool { return false })
	b := &loopBatchCoordinator{
		ctx:       context.Background(),
		cfg:       loopConfig{ProjectRoot: root, EvolveDir: evolveDir},
		result:    &loopResult{},
		stdout:    io.Discard,
		stderr:    io.Discard,
		loadGraph: noModuleGraph,
	}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, io.Discard)
	b.deps.Storage = &fixtures.FakeStorage{}
	b.preWave = func(int) (batchDecision, bool) { return batchDecision{flow: batchProceed}, false }
	return b
}

func TestPrepareIteration_ZeroShip_AWaveIsTheCyclesBetweenTwoIterationBoundaries(t *testing.T) {
	h := newZW(t)
	fc, bin := policy.FleetConfig{Count: 2}, ""
	prepare := func(iteration, seconds int) batchDecision {
		h.now = h.at(seconds)
		return h.b.prepareIteration(iteration, &fc, &bin, zwFloor)
	}

	if d := prepare(0, 1); d.flow != batchProceed {
		t.Fatalf("iteration 0 launches: no lane has run yet: %+v", d)
	}
	h.cycle(11)
	h.cycle(12)
	if d := prepare(1, 60); d.flow != batchProceed {
		t.Fatalf("one zero-ship fleet wave is below the ceiling: %+v", d)
	}
	h.cycle(13)
	h.cycle(14)
	h.ship(14, h.at(90))
	if d := prepare(2, 120); d.flow != batchProceed {
		t.Fatalf("a wave with one ship among its lanes is a productive wave and resets the count: %+v", d)
	}
	h.cycle(15)
	h.cycle(16)
	if d := prepare(3, 180); d.flow != batchProceed {
		t.Fatalf("one zero-ship wave after the ship is below the ceiling: %+v", d)
	}
	h.cycle(17)
	h.cycle(18)
	d := prepare(4, 240)
	if d.flow != batchReturn || d.exitCode != systemFailureHaltExitCode {
		t.Fatalf("two consecutive zero-ship fleet waves halt before iteration 4 launches: %+v", d)
	}
	if h.b.result.StopReason == "" {
		t.Error("the halt records a stop reason on the loop result")
	}
}

func TestPrepareIteration_ZeroShip_AnIterationThatLaunchedNothingIsNotAWave(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	b := zsCoordinator(t, evolveDir, root)
	fc, bin := policy.FleetConfig{Count: 1}, ""

	for iteration := 0; iteration < 4; iteration++ {
		if d := b.prepareIteration(iteration, &fc, &bin, 0); d.flow != batchProceed {
			t.Fatalf("iteration %d launched no cycle, so it is no zero-ship wave: %+v", iteration, d)
		}
	}
}

func TestZeroShipHalt_UnreadableFailurePolicyWarnsAndKeepsTheCompiledDefault(t *testing.T) {
	h := newZW(t)
	badPolicy := `{"failure_policy":{"categories":{"bogus":{"level":"nope","action":"nope"}}}}`
	h.write(".evolve/policy.json", badPolicy)
	h.wantNoHalt(1)
	h.cycle(11)

	h.wantNoHalt(60)
	if !strings.Contains(h.stderr.String(), "failure policy unreadable") {
		t.Errorf("an unreadable failure policy warns on stderr:\n%s", h.stderr.String())
	}
	h.cycle(12)
	if _, halted := h.boundary(120); !halted {
		t.Error("the compiled default ceiling of two still halts on the second zero-ship wave")
	}
}

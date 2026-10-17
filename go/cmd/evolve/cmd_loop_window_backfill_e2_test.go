//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

var e2PoolFleet = policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}

type e2WindowFixture struct {
	root, evolveDir string
	b               *loopBatchCoordinator
	console         *bytes.Buffer
}

func newE2WindowFixture(t *testing.T, items map[string]string) e2WindowFixture {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	evolveDir := t.TempDir()
	for name, body := range items {
		writeE2File(t, filepath.Join(evolveDir, "inbox", name), body)
	}
	b, console := iscsCoordinator(repo.Dir, evolveDir, &erroringReadStorage{})
	b.cfg.GoalHash = "e2-window"
	return e2WindowFixture{root: repo.Dir, evolveDir: evolveDir, b: b, console: console}
}

var e2TwoItems = map[string]string{
	"a.json": `{"id":"alpha","weight":0.9,"priority_class":"feature","files":["a.go"]}`,
	"b.json": `{"id":"beta","weight":0.8,"priority_class":"feature","files":["b.go"]}`,
}

func TestDispatchPool_RunsTheRollingPoolAndContinues(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, e2TwoItems)
	bin := fakeCycleBinary(t, "exit 0\n")

	d, done := f.b.dispatchPool(0, e2PoolFleet, bin)
	f.b.deps.Signals.Flush()

	if !done || d.flow != batchNextIteration {
		t.Fatalf("done=%v decision=%+v console:\n%s", done, d, f.console.String())
	}
	if !strings.Contains(f.console.String(), "[loop] pool 0: 2/2 lanes ok (rolling, target=2)") {
		t.Fatalf("console:\n%s", f.console.String())
	}
}

func TestDispatchPool_ASystemFailureLaneHaltsTheBatch(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, e2TwoItems)
	bin := fakeCycleBinary(t, "exit 4\n")

	d, done := f.b.dispatchPool(3, e2PoolFleet, bin)
	f.b.deps.Signals.Flush()

	if !done || d.flow != batchReturn || d.exitCode != systemFailureHaltExitCode {
		t.Fatalf("done=%v decision=%+v console:\n%s", done, d, f.console.String())
	}
	if f.b.result.StopReason != "system_failure_halt" {
		t.Fatalf("stop reason = %q", f.b.result.StopReason)
	}
}

func TestDispatchPool_FallsBackToSequential(t *testing.T) {
	t.Parallel()
	t.Run("an empty backlog", func(t *testing.T) {
		f := newE2WindowFixture(t, nil)
		f.b.deps.Storage = &fixtures.FakeStorage{State: core.State{LastCycleNumber: 7}}
		writeE2File(t, filepath.Join(cycleWorkspace(f.root, 7), "triage-decision.json"), `{"top_n":[]}`)
		d, done := f.b.dispatchPool(1, e2PoolFleet, "/nonexistent-binary")
		if done || d.flow != batchProceed || !strings.Contains(f.console.String(), "[loop] WARN: fleet: pool 1 planned zero lanes (empty backlog), falling back to sequential") {
			t.Fatalf("done=%v decision=%+v console:\n%s", done, d, f.console.String())
		}
	})
	t.Run("an unverifiable control plane", func(t *testing.T) {
		f := newE2WindowFixture(t, e2TwoItems)
		f.b.cfg.ProjectRoot = t.TempDir()
		d, done := f.b.dispatchPool(2, e2PoolFleet, "/nonexistent-binary")
		if done || d.flow != batchProceed || !strings.Contains(f.console.String(), "[loop] WARN: fleet: pool 2 dispatch failed, falling back to sequential: pool 2: control-plane preflight:") {
			t.Fatalf("done=%v decision=%+v console:\n%s", done, d, f.console.String())
		}
	})
}

func TestDispatchFleetIteration_APoolIterationEndsTheIteration(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, e2TwoItems)
	bin := fakeCycleBinary(t, "exit 0\n")

	d := f.b.dispatchFleetIteration(0, e2PoolFleet, bin, &fleet.StarvationTracker{})

	if d.flow != batchNextIteration {
		t.Fatalf("decision=%+v console:\n%s", d, f.console.String())
	}
}

func TestDispatchFleetIteration_AWaveThatRanCompletesTheWave(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, e2TwoItems)
	bin := fakeCycleBinary(t, "exit 0\n")

	d := f.b.dispatchFleetIteration(0, policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}, bin, &fleet.StarvationTracker{})
	f.b.deps.Signals.Flush()

	if d.flow != batchNextIteration || !strings.Contains(f.console.String(), "lanes ok") {
		t.Fatalf("decision=%+v console:\n%s", d, f.console.String())
	}
}

func TestRepairMinWidth_ARepairedLaneEndsTheIteration(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, e2TwoItems)
	bin := fakeCycleBinary(t, "exit 0\n")
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}

	d := f.b.repairMinWidth(0, fc, policy.FleetConfig{Count: 0, PlanSource: "triage"}, bin, &fleet.StarvationTracker{})
	f.b.deps.Signals.Flush()

	if d.flow != batchNextIteration || !strings.Contains(f.console.String(), "min-width repair dispatched") {
		t.Fatalf("decision=%+v console:\n%s", d, f.console.String())
	}
}

func TestCompleteWave_ASystemFailureLaneHaltsBeforeTheBoundary(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, nil)

	d := f.b.completeWave(4, policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 2}, []fleet.Result{{Index: 0}, {Index: 1, ExitCode: systemFailureHaltExitCode}}, 0, &fleet.StarvationTracker{})
	f.b.deps.Signals.Flush()

	if d.flow != batchReturn || d.exitCode != systemFailureHaltExitCode || f.b.result.StopReason != "system_failure_halt" {
		t.Fatalf("decision=%+v stop=%q console:\n%s", d, f.b.result.StopReason, f.console.String())
	}
	if _, err := os.Stat(filepath.Join(f.evolveDir, "escalation-apply-report.json")); !os.IsNotExist(err) {
		t.Fatalf("a halted wave applied the escalation boundary: %v", err)
	}
}

func TestObserveWorkSupply_AnUnwritableStarvationTodoWarns(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, nil)
	f.b.cfg.EvolveDir = filepath.Join(t.TempDir(), "evolve-as-file")
	writeE2File(t, f.b.cfg.EvolveDir, "x")
	tracker := &fleet.StarvationTracker{}

	f.b.observeWorkSupply(2, policy.FleetConfig{Count: 3, StarvationK: 1, StarvationWeight: 0.5}, policy.FleetConfig{Count: 3}, 0, tracker)

	if !strings.Contains(f.console.String(), "[loop] WARN: fleet: could not self-file starvation todo: ") {
		t.Fatalf("console:\n%s", f.console.String())
	}
}

func TestRepairedLanes_IsTheRepairWidthOnlyWhenRepaired(t *testing.T) {
	t.Parallel()
	if repairedLanes(false) != 0 || repairedLanes(true) <= 0 {
		t.Fatalf("repairedLanes(false)=%d repairedLanes(true)=%d", repairedLanes(false), repairedLanes(true))
	}
}

func TestPrepareIteration_ACancelledBatchStopsBeforeAnyBoundaryWork(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.b.ctx = ctx
	fc, bin := policy.FleetConfig{Count: 1}, ""

	d := f.b.prepareIteration(2, &fc, &bin, 0)

	if d.flow != batchReturn || d.exitCode != 130 || !strings.Contains(f.console.String(), "before cycle 3") {
		t.Fatalf("decision=%+v console:\n%s", d, f.console.String())
	}
}

func TestProbeSyncAndPublish_AnInterruptDuringTheProbesStops(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, nil)
	f.b.cycleEnv = map[string]string{"EVOLVE_CLI_HEALTH": "0"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.b.ctx = ctx

	d, stop := f.b.probeSyncAndPublish(0)

	if !stop || d.flow != batchReturn || d.exitCode != 130 || !strings.Contains(f.console.String(), "during the pre-wave probes before cycle 1") {
		t.Fatalf("stop=%v decision=%+v console:\n%s", stop, d, f.console.String())
	}
}

func TestResolveWaveBinary_AnUnresolvableBinaryStaysSequential(t *testing.T) {
	f := newE2WindowFixture(t, nil)
	e2StubFleetExecutable(t, "", errors.New("exe gone"))
	fc, bin := policy.FleetConfig{Count: 3, PlanSource: "triage"}, ""

	f.b.resolveWaveBinary(&fc, &bin)

	if fc.Count != 1 || bin != "" || !strings.Contains(f.console.String(), "[loop] WARN: fleet: cannot resolve binary for fleet dispatch, staying sequential: exe gone") {
		t.Fatalf("count=%d bin=%q console:\n%s", fc.Count, bin, f.console.String())
	}
}

func runE2PoolBatch(t *testing.T, laneBody string, extra ...string) (rc int, stdout, stderr string) {
	t.Helper()
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv(bridge.TmuxSocketEnv, "evolve-bridge-e2run")
	t.Setenv("EVOLVE_CLI_HEALTH", "0")
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeE2File(t, filepath.Join(evolveDir, "policy.json"), `{"fleet":{"count":2,"concurrency":2,"plan_source":"triage","scheduling":"pool","landing":"bogus"},"workflow":{"cycle_budget":"enforce","max_cycles_cap":1}}`)
	for name, body := range e2TwoItems {
		writeE2File(t, filepath.Join(evolveDir, "inbox", name), body)
	}
	installGCHookSpy(t, evolveDir)
	defer installStubDeps(t, &fixtures.FakeStorage{}, newFakeLedger())()
	e2StubFleetExecutable(t, fakeCycleBinary(t, laneBody), nil)
	var out, errb bytes.Buffer
	args := append([]string{"--project-root", projectRoot, "--evolve-dir", evolveDir, "--goal-text", "e2 pool batch"}, extra...)
	rc = runLoop(args, nil, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestLoopBatchRun_ABudgetedPoolBatchRunsOneRollingIteration(t *testing.T) {
	rc, stdout, stderr := runE2PoolBatch(t, "exit 0\n")

	for _, want := range []string{
		"[loop] no --cycles: advisor-decided, completion-driven (stops when the backlog drains), safety cap=1",
		`[loop] WARN: fleet: fleet.landing: unknown value "bogus"`,
		"[loop] pool 0: 2/2 lanes ok (rolling, target=2)",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr lacks %q (rc=%d):\n%s", want, rc, stderr)
		}
	}
	if strings.Contains(stderr, "pool 1:") {
		t.Fatalf("the safety cap of 1 ran a second iteration:\n%s", stderr)
	}
	if !strings.Contains(stdout, `"stop_reason"`) {
		t.Fatalf("the batch must emit its result: %s", stdout)
	}
}

func TestLoopBatchRun_ASystemFailureLaneReturnsItsHaltCode(t *testing.T) {
	rc, _, stderr := runE2PoolBatch(t, "exit 4\n", "--cycles", "3")

	if rc != systemFailureHaltExitCode {
		t.Fatalf("rc=%d, want %d:\n%s", rc, systemFailureHaltExitCode, stderr)
	}
	if strings.Contains(stderr, "pool 1:") {
		t.Fatalf("a halted batch dispatched another pool:\n%s", stderr)
	}
}

func TestDispatchPool_AtEnforceTheOrchestratorRunsTheLanesAndPublishesEachTransition(t *testing.T) {
	t.Parallel()
	f := newE2WindowFixture(t, e2TwoItems)
	bin := fakeCycleBinary(t, "exit 0\n")
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave",
		Orchestrator: policy.OrchestratorConfig{Stage: "enforce", ScanK: 32, HubMinImporters: 120, DeadlineFloorMinutes: 60, StallMinutes: 20, RunMaxLanes: 8}}

	d, done := f.b.dispatchPool(0, fc, bin)
	f.b.deps.Signals.Flush()

	if !done || d.flow != batchNextIteration {
		t.Fatalf("done=%v decision=%+v console:\n%s", done, d, f.console.String())
	}
	if !strings.Contains(f.console.String(), "[loop] pool 0: 2/2 lanes ok (orchestrator, target=2)") {
		t.Fatalf("console:\n%s", f.console.String())
	}
	raw, err := os.ReadFile(filepath.Join(f.evolveDir, "signals.ndjson"))
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"LOOP_ORCH_LANE_LAUNCHED", "LOOP_ORCH_LANE_EXITED", "LOOP_ORCH_POOL_IDLE"} {
		if !strings.Contains(string(raw), `"code":"`+code+`"`) {
			t.Errorf("signals lack %s:\n%s", code, raw)
		}
	}
}

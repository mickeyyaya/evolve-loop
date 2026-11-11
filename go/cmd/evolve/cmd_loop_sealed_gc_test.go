package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const sealGCLeafPrefix = "cycle-cd3ae73e-"

type sealGCRunner struct {
	cycle int
	onRun func()
}

func (r sealGCRunner) finish() (core.CycleResult, error) {
	if r.onRun != nil {
		r.onRun()
	}
	return core.CycleResult{Cycle: r.cycle}, nil
}

func (r sealGCRunner) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	return r.finish()
}

func (r sealGCRunner) RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error) {
	return r.finish()
}

func (sealGCRunner) SignalSummary() signalcenter.Summary { return signalcenter.Summary{} }

type sealGCEnv struct {
	t           *testing.T
	projectRoot string
	evolveDir   string
	signaled    []int
}

func newSealGCEnv(t *testing.T, mode string) *sealGCEnv {
	t.Helper()
	projectRoot, evolveDir, _ := gcWorktreeEnv(t, mode)
	e := &sealGCEnv{t: t, projectRoot: projectRoot, evolveDir: evolveDir}
	prevHost := gcInjectedProcessHost
	gcInjectedProcessHost = &gcProcessHost{
		list:   prevHost.list,
		signal: func(pid int, _ syscall.Signal) error { e.signaled = append(e.signaled, pid); return nil },
		sleep:  prevHost.sleep,
		alive:  prevHost.alive,
	}
	t.Cleanup(func() { gcInjectedProcessHost = prevHost })
	return e
}

func (e *sealGCEnv) lane(cycle int, dirty bool) string {
	e.t.Helper()
	leaf := fmt.Sprintf("%s%d", sealGCLeafPrefix, cycle)
	wt := filepath.Join(e.projectRoot, ".evolve", "worktrees", leaf)
	gcGit(e.t, e.projectRoot, "worktree", "add", "-q", wt, "-b", leaf)
	if err := os.WriteFile(filepath.Join(wt, "lane.txt"), []byte("lane work\n"), 0o644); err != nil {
		e.t.Fatal(err)
	}
	gcGit(e.t, wt, "add", "lane.txt")
	gcGit(e.t, wt, "commit", "-q", "-m", "lane work")
	if dirty {
		if err := os.WriteFile(filepath.Join(wt, "seed.txt"), []byte("seed\nuncommitted edit\n"), 0o644); err != nil {
			e.t.Fatal(err)
		}
	}
	return wt
}

func (e *sealGCEnv) orphansIn(cwds ...string) {
	e.t.Helper()
	var script string
	for i, cwd := range cwds {
		script += fmt.Sprintf("printf 'p%d\\nR1\\nfcwd\\nn%s\\n'\n", 20+i, cwd)
	}
	bin := e.t.TempDir()
	fakeclitest.Install(e.t, filepath.Join(bin, "lsof"), "#!/bin/sh\n"+script)
	e.t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func (e *sealGCEnv) planeState(state map[string]any) {
	e.t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.evolveDir, core.CycleStateFile), raw, 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *sealGCEnv) pausedPlane(cycle int) {
	e.t.Helper()
	st := storage.New(e.evolveDir)
	ctx := context.Background()
	cs := core.CycleState{CycleID: cycle, RunID: "lane-run", Phase: "build", WorkspacePath: cycleWorkspace(e.projectRoot, cycle)}
	if err := st.WriteState(ctx, core.State{LastCycleNumber: cycle}); err != nil {
		e.t.Fatal(err)
	}
	if err := st.WriteCycleState(ctx, cs); err != nil {
		e.t.Fatal(err)
	}
	if err := checkpoint.ApplyToStateFile(filepath.Join(e.evolveDir, core.CycleStateFile), checkpoint.Compose(cs, checkpoint.ReasonQuotaLikely, 0, "", time.Now())); err != nil {
		e.t.Fatal(err)
	}
}

func (e *sealGCEnv) sealedPlane(cycle int, lane string) {
	e.t.Helper()
	e.planeState(map[string]any{"cycle_id": cycle, "phase": "end", "active_worktree": lane})
	runDir := filepath.Join(e.evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), []byte(`{"active_worktree":"`+lane+`"}`), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *sealGCEnv) sequentialCycle(cycle int) string {
	e.t.Helper()
	var stderr bytes.Buffer
	b := &loopBatchCoordinator{
		ctx:    context.Background(),
		cfg:    loopConfig{ProjectRoot: e.projectRoot, EvolveDir: e.evolveDir},
		orch:   sealGCRunner{cycle: cycle},
		deps:   orchDeps{Storage: &fixtures.FakeStorage{}},
		result: &loopResult{},
		stdout: io.Discard,
		stderr: &stderr,
	}
	if _, decision := b.dispatchSequentialCycle(); decision.flow != batchProceed {
		e.t.Fatalf("dispatchSequentialCycle flow = %v, want proceed; stderr=%s", decision.flow, stderr.String())
	}
	return stderr.String()
}

func (e *sealGCEnv) assertKept(lane string, cycle int) {
	e.t.Helper()
	if _, err := os.Stat(lane); err != nil {
		e.t.Errorf("lane of cycle %d must stay: %v", cycle, err)
	}
	if !gcBranchExists(e.t, e.projectRoot, fmt.Sprintf("%s%d", sealGCLeafPrefix, cycle)) {
		e.t.Errorf("branch of cycle %d must stay", cycle)
	}
}

func (e *sealGCEnv) assertReleased(lane string, cycle int) {
	e.t.Helper()
	if _, err := os.Stat(lane); !os.IsNotExist(err) {
		e.t.Errorf("lane of the sealed cycle %d must be released at once: %v", cycle, err)
	}
	if gcBranchExists(e.t, e.projectRoot, fmt.Sprintf("%s%d", sealGCLeafPrefix, cycle)) {
		e.t.Errorf("branch of the sealed cycle %d must be released", cycle)
	}
	if !gcBranchExists(e.t, e.projectRoot, gcOrphanBranch) {
		e.t.Errorf("branch %s belongs to no sealed cycle and must stay", gcOrphanBranch)
	}
}

func TestDispatchSequentialCycle_ASealedCycleHasItsDirtyLaneBranchAndOrphanCleanedAtOnce(t *testing.T) {
	e := newSealGCEnv(t, "enforce")
	sealed := e.lane(800, true)
	other := e.lane(801, true)
	e.sealedPlane(800, sealed)
	e.orphansIn(filepath.Join(sealed, "go"), other)

	e.sequentialCycle(800)

	e.assertReleased(sealed, 800)
	e.assertKept(other, 801)
	if !reflect.DeepEqual(e.signaled, []int{20}) {
		t.Errorf("signaled %v, want [20]: the orphan of the sealed lane, never the orphan of cycle 801", e.signaled)
	}
}

func TestDispatchSequentialCycle_TheGCModeGovernsTheSealedCycleSweep(t *testing.T) {
	for _, mode := range []string{"shadow", "off"} {
		t.Run(mode, func(t *testing.T) {
			e := newSealGCEnv(t, mode)
			sealed := e.lane(800, true)
			e.sealedPlane(800, sealed)
			e.orphansIn(sealed)

			e.sequentialCycle(800)

			e.assertKept(sealed, 800)
			if len(e.signaled) != 0 {
				t.Errorf("gc.mode %s must signal nothing, signaled %v", mode, e.signaled)
			}
		})
	}
}

func TestDispatchSequentialCycle_ACycleThatSealsIntoAPauseKeepsItsLaneBranchAndProcesses(t *testing.T) {
	e := newSealGCEnv(t, "enforce")
	lane := e.lane(800, true)
	e.planeState(map[string]any{"cycle_id": 800, "phase": "build", "active_worktree": lane, "checkpoint": map[string]any{
		"enabled": true, "reason": "quota-likely", "resumeFromPhase": "build", "worktreePath": lane,
		"savedAt": time.Now().UTC().Format(time.RFC3339), "autoResumeAttempts": 0, "autoResumeMaxAttempts": 3,
	}})
	e.orphansIn(lane)

	e.sequentialCycle(800)

	e.assertKept(lane, 800)
	if len(e.signaled) != 0 {
		t.Errorf("a paused cycle keeps its processes, signaled %v", e.signaled)
	}
}

func TestDispatchSequentialCycle_TheLiveLaneOfAnotherCycleIsNotTouched(t *testing.T) {
	e := newSealGCEnv(t, "enforce")
	sealed := e.lane(800, true)
	live := e.lane(801, true)
	e.sealedPlane(800, sealed)
	liveRun := filepath.Join(e.evolveDir, "runs", "cycle-801")
	if err := os.MkdirAll(liveRun, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(liveRun, runlease.Lease{RunID: "cycle-801", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	e.orphansIn(live)

	e.sequentialCycle(800)

	e.assertKept(live, 801)
	if len(e.signaled) != 0 {
		t.Errorf("the orphan sits in the live lane of cycle 801, signaled %v", e.signaled)
	}
}

func TestRunResumeBatch_AResumedCycleThatSealsHasItsLaneBranchAndOrphanCleanedAtOnce(t *testing.T) {
	e := newSealGCEnv(t, "enforce")
	e.pausedPlane(7)
	sealed := e.lane(7, true)
	e.orphansIn(sealed)
	runner := sealGCRunner{cycle: 7, onRun: func() { e.sealedPlane(7, sealed) }}
	var out, errOut bytes.Buffer
	lr := loopResult{}

	rc := runResumeBatch(context.Background(), loopConfig{ProjectRoot: e.projectRoot, EvolveDir: e.evolveDir, Resume: true},
		runner, newFakeLedger(), nil, map[string]string{}, map[string]string{}, &lr, &out, &errOut)

	if rc != 0 {
		t.Fatalf("rc=%d, stderr:\n%s", rc, errOut.String())
	}
	e.assertReleased(sealed, 7)
	if !reflect.DeepEqual(e.signaled, []int{20}) {
		t.Errorf("signaled %v, want [20]", e.signaled)
	}
}

func (e *sealGCEnv) productionPidCheck() {
	e.t.Helper()
	gcInjectedProcessHost.alive = runlease.PIDAlive
}

func (e *sealGCEnv) leaseLikeTheOrchestrator(cycle int) func() {
	e.t.Helper()
	return func() {
		ws := cycleWorkspace(e.projectRoot, cycle)
		if err := os.MkdirAll(ws, 0o755); err != nil {
			e.t.Fatal(err)
		}
		if err := runlease.Write(ws, runlease.Lease{RunID: fmt.Sprintf("cycle-%d", cycle), OwnerPID: os.Getpid()}, time.Now()); err != nil {
			e.t.Fatal(err)
		}
	}
}

func TestDispatchSequentialCycle_ASealedCycleWhoseOrchestratorLeftAFreshLeaseOwnedByTheLoopIsStillCleanedAtOnce(t *testing.T) {
	e := newSealGCEnv(t, "enforce")
	e.productionPidCheck()
	sealed := e.lane(800, true)
	other := e.lane(801, true)
	e.sealedPlane(800, sealed)
	e.orphansIn(sealed, other)
	var stderr bytes.Buffer
	b := &loopBatchCoordinator{
		ctx:    context.Background(),
		cfg:    loopConfig{ProjectRoot: e.projectRoot, EvolveDir: e.evolveDir},
		orch:   sealGCRunner{cycle: 800, onRun: e.leaseLikeTheOrchestrator(800)},
		deps:   orchDeps{Storage: &fixtures.FakeStorage{}},
		result: &loopResult{},
		stdout: io.Discard,
		stderr: &stderr,
	}

	b.dispatchSequentialCycle()

	e.assertReleased(sealed, 800)
	e.assertKept(other, 801)
	if !reflect.DeepEqual(e.signaled, []int{20}) {
		t.Errorf("signaled %v, want [20]; stderr=%s", e.signaled, stderr.String())
	}
}

func TestRunResumeBatch_AResumedCycleWhoseOrchestratorLeftAFreshLeaseOwnedByTheLoopIsStillCleanedAtOnce(t *testing.T) {
	e := newSealGCEnv(t, "enforce")
	e.productionPidCheck()
	e.pausedPlane(7)
	sealed := e.lane(7, true)
	e.orphansIn(sealed)
	lease := e.leaseLikeTheOrchestrator(7)
	runner := sealGCRunner{cycle: 7, onRun: func() { lease(); e.sealedPlane(7, sealed) }}
	var out, errOut bytes.Buffer
	lr := loopResult{}

	rc := runResumeBatch(context.Background(), loopConfig{ProjectRoot: e.projectRoot, EvolveDir: e.evolveDir, Resume: true},
		runner, newFakeLedger(), nil, map[string]string{}, map[string]string{}, &lr, &out, &errOut)

	if rc != 0 {
		t.Fatalf("rc=%d, stderr:\n%s", rc, errOut.String())
	}
	e.assertReleased(sealed, 7)
	if !reflect.DeepEqual(e.signaled, []int{20}) {
		t.Errorf("signaled %v, want [20]; stderr=%s", e.signaled, errOut.String())
	}
}

func TestDispatchSequentialCycle_ASealedCycleWhoseFreshLeaseIsOwnedByAnotherLiveProcessKeepsItsLane(t *testing.T) {
	e := newSealGCEnv(t, "enforce")
	e.productionPidCheck()
	lane := e.lane(800, true)
	e.sealedPlane(800, lane)
	e.orphansIn(lane)
	foreignLease := func() {
		if err := runlease.Write(cycleWorkspace(e.projectRoot, 800), runlease.Lease{RunID: "cycle-800", OwnerPID: os.Getppid()}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	b := &loopBatchCoordinator{
		ctx:    context.Background(),
		cfg:    loopConfig{ProjectRoot: e.projectRoot, EvolveDir: e.evolveDir},
		orch:   sealGCRunner{cycle: 800, onRun: foreignLease},
		deps:   orchDeps{Storage: &fixtures.FakeStorage{}},
		result: &loopResult{},
		stdout: io.Discard,
		stderr: io.Discard,
	}

	b.dispatchSequentialCycle()

	e.assertKept(lane, 800)
	if len(e.signaled) != 0 {
		t.Errorf("another live process owns the run, signaled %v", e.signaled)
	}
}

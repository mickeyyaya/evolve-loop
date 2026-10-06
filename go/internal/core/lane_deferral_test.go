package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const provisionCause = "worktree base ref (cycle 1806): git fetch origin main: rc=1 err=<nil>: error: cannot lock ref 'refs/remotes/origin/main': is at e1d9 but expected 0bc8"

type flakyWorktree struct {
	fakeWorktree
	failures int
}

func (f *flakyWorktree) Create(projectRoot string, cycle int) (string, error) {
	if f.failures > 0 {
		f.failures--
		f.createdCycles = append(f.createdCycles, cycle)
		return "", errors.New(provisionCause)
	}
	return f.fakeWorktree.Create(projectRoot, cycle)
}

func pendingInboxItem(t *testing.T, root, id string) string {
	t.Helper()
	path := filepath.Join(root, ".evolve", "inbox", id+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"id":"`+id+`","weight":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func dispatchedPhases(runners map[Phase]PhaseRunner) []Phase {
	var out []Phase
	for p, r := range runners {
		if len(r.(*fakeRunner).requests) > 0 {
			out = append(out, p)
		}
	}
	return out
}

func TestRunCycle_AFleetLaneWithoutAWorktreeDefersBeforeAnyDispatch(t *testing.T) {
	c, got := recordingCenter()
	st := &fakeStorage{state: State{LastCycleNumber: 1805}}
	runners := buildRunners(nil)
	wt := &flakyWorktree{failures: 99}
	root := t.TempDir()
	item := pendingInboxItem(t, root, "inboxbatch-utf8-and-resolution")
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(wt), WithSignalCenter(c))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", Env: map[string]string{ipcenv.FleetKey: "1"}})

	var deferral *LaneDeferral
	if !errors.As(err, &deferral) || deferral.Cycle != 1806 || !strings.Contains(err.Error(), "cannot lock ref") {
		t.Fatalf("err = %v, want a *LaneDeferral for cycle 1806 carrying the provisioning cause", err)
	}
	if len(wt.createdCycles) != 2 {
		t.Errorf("Create calls = %v, want the first attempt and one re-provision", wt.createdCycles)
	}
	if phases := dispatchedPhases(runners); len(phases) != 0 {
		t.Errorf("phases dispatched without a worktree: %v — in fleet mode the bridge refuses each launch and the cycle seals FAIL", phases)
	}
	ws := RunWorkspacePath(root, 1806)
	if oc, detail := cyclehealth.ClassifyOutcome(ws); oc != cyclehealth.OutcomeDeferred {
		t.Errorf("outcome = %s (%s), want DEFERRED", oc, detail)
	}
	for _, f := range []string{"failure-digest.json", "audit-fail-reason.json"} {
		if _, err := os.Stat(filepath.Join(ws, f)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s exists (err=%v): an infrastructure deferral must leave no failure evidence about the task", f, err)
		}
	}
	for _, s := range st.stateLog {
		if len(s.CarryoverTodos) > 0 || len(s.FailedAt) > 0 {
			t.Errorf("failure learning ran: carryover=%v failed=%v", s.CarryoverTodos, s.FailedAt)
		}
	}
	if _, err := os.Stat(item); err != nil {
		t.Errorf("the lane's item left the queue: %v", err)
	}
	sealed := eventsOfKind(*got, signalcenter.KindCycleSealed)
	if len(sealed) != 1 {
		t.Fatalf("cycle.sealed events = %+v, want exactly one", sealed)
	}
	e := sealed[0]
	if e.Code != CodeLaneDeferred || e.Severity != signalcenter.SeverityWarn || e.Cycle != 1806 || e.RunID == "" ||
		e.Fields["step"] != "worktree" || !strings.Contains(e.Fields["cause"], "cannot lock ref") {
		t.Errorf("the seal is a registered deferral code naming the step and the cause: %+v", e)
	}
	if m, ok := signalcenter.IsRegistered(CodeLaneDeferred); !ok || m != signalcenter.ModuleOrchestrator {
		t.Errorf("%s is not registered to the orchestrator (module=%q)", CodeLaneDeferred, m)
	}
}

func TestRunCycle_AReprovisionedFleetLaneRunsNormally(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 1805}}
	runners := buildRunners(nil)
	wt := &flakyWorktree{fakeWorktree: fakeWorktree{path: t.TempDir()}, failures: 1}
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(wt))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", Env: map[string]string{ipcenv.FleetKey: "1"}})

	var deferral *LaneDeferral
	if errors.As(err, &deferral) {
		t.Fatalf("a provision that succeeds on its retry deferred the lane: %v", err)
	}
	if len(wt.createdCycles) != 2 {
		t.Errorf("Create calls = %v, want one failure and one successful re-provision", wt.createdCycles)
	}
	if req := runners[PhaseBuild].(*fakeRunner).requests; len(req) == 0 || req[0].Worktree != wt.path {
		t.Errorf("build requests = %+v, want the re-provisioned worktree %s", req, wt.path)
	}
}

func TestRunCycle_ASequentialCycleWithoutAWorktreeKeepsItsBestEffortRun(t *testing.T) {
	c, got := recordingCenter()
	st := &fakeStorage{state: State{LastCycleNumber: 1805}}
	runners := buildRunners(nil)
	wt := &flakyWorktree{failures: 99}
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(wt), WithSignalCenter(c))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})

	if err != nil {
		t.Fatalf("a sequential cycle keeps running without a worktree: %v", err)
	}
	if len(wt.createdCycles) != 1 {
		t.Errorf("Create calls = %v, want one: the re-provision belongs to the fleet deferral, a sequential cycle runs on without it", wt.createdCycles)
	}
	if len(runners[PhaseScout].(*fakeRunner).requests) == 0 {
		t.Error("scout never ran: sequential phases still dispatch, only source writes are refused")
	}
	if !workspaceMentions(t, st.cycleState.WorkspacePath, "worktree provisioning failed") {
		t.Error("the sequential failure record no longer names the provisioning cause")
	}
	for _, e := range *got {
		if e.Code == CodeLaneDeferred {
			t.Errorf("a sequential cycle emitted the fleet deferral: %+v", e)
		}
	}
}

func TestLaneDeferral_NamesTheCycleAndUnwrapsToItsCause(t *testing.T) {
	cause := errors.New(provisionCause)
	d := &LaneDeferral{Cycle: 1806, Cause: cause}
	if d.Unwrap() != cause || !errors.Is(d, cause) {
		t.Errorf("a deferral must unwrap to the provisioning cause so callers can still classify it")
	}
	if msg := d.Error(); !strings.Contains(msg, "cycle 1806") || !strings.Contains(msg, "cannot lock ref") {
		t.Errorf("Error() = %q, want the cycle and the cause", msg)
	}
}

func TestRunCycle_ADeferralThatCannotBeRecordedFailsLoudly(t *testing.T) {
	c, got := recordingCenter()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "runs"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 1805}}, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&flakyWorktree{failures: 99}), WithSignalCenter(c))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", Env: map[string]string{ipcenv.FleetKey: "1"}})

	var deferral *LaneDeferral
	if err == nil || errors.As(err, &deferral) || !strings.Contains(err.Error(), "not recorded") || !strings.Contains(err.Error(), "cannot lock ref") {
		t.Fatalf("err = %v, want a loud non-deferral error naming the unrecorded deferral and the provisioning cause: an unrecorded deferral is invisible to the breaker", err)
	}
	for _, e := range *got {
		if e.Code == CodeLaneDeferred {
			t.Errorf("a deferral that left no record must not seal as one: %+v", e)
		}
	}
}

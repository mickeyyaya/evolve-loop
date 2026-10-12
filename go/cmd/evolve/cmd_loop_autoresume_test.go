package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type countingQuotaRunner struct{ calls *int }

func (r countingQuotaRunner) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	return core.CycleResult{}, errors.New("RunCycle is not a resume")
}

func (r countingQuotaRunner) RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error) {
	*r.calls++
	return core.CycleResult{Cycle: 7}, core.ErrAllFamiliesExhausted
}

func (countingQuotaRunner) SignalSummary() signalcenter.Summary { return signalcenter.Summary{} }

func seedQuotaPause(t *testing.T, attempts int) (root, evolveDir string) {
	t.Helper()
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	st := storage.New(evolveDir)
	ctx := context.Background()
	cs := core.CycleState{CycleID: 7, RunID: "paused-run", Phase: "audit", WorkspacePath: cycleWorkspace(root, 7)}
	if err := st.WriteState(ctx, core.State{LastCycleNumber: 7}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteCycleState(ctx, cs); err != nil {
		t.Fatal(err)
	}
	cp := checkpoint.Compose(cs, checkpoint.ReasonQuotaLikely, 0, "", time.Now())
	cp.QuotaResetAt, cp.QuotaResetSource, cp.AutoResumeAttempts = time.Now().Add(time.Hour).Format(time.RFC3339), "bench", attempts
	if err := checkpoint.ApplyToStateFile(filepath.Join(evolveDir, core.CycleStateFile), cp); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func resumeOnce(t *testing.T, root, evolveDir string, auto bool) (rc, calls int, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	lr := loopResult{}
	cfg := loopConfig{ProjectRoot: root, EvolveDir: evolveDir, Resume: true, AutoResume: auto}
	rc = runResumeBatch(context.Background(), cfg, countingQuotaRunner{calls: &calls}, newFakeLedger(), nil, map[string]string{}, map[string]string{}, &lr, &out, &errOut)
	return rc, calls, errOut.String()
}

func attemptsOnDisk(t *testing.T, evolveDir string) int {
	t.Helper()
	qp, ok := detectQuotaPause(evolveDir)
	if !ok {
		t.Fatal("the quota-likely checkpoint is gone")
	}
	return qp.Attempts
}

func TestRunResumeBatch_AnAutoResumeCountsAndTheOneAtTheCapIsRefusedWithTheOperatorAction(t *testing.T) {
	root, evolveDir := seedQuotaPause(t, 0)
	for attempt := 1; attempt <= checkpoint.DefaultAutoResumeAttempts; attempt++ {
		if rc, calls, stderr := resumeOnce(t, root, evolveDir, true); rc != 5 || calls != 1 {
			t.Fatalf("auto-resume %d: rc=%d calls=%d, want the cycle re-run into the wall (rc 5)\n%s", attempt, rc, calls, stderr)
		}
		if got := attemptsOnDisk(t, evolveDir); got != attempt {
			t.Fatalf("after auto-resume %d attempts=%d, want %d", attempt, got, attempt)
		}
	}

	rc, calls, stderr := resumeOnce(t, root, evolveDir, true)

	if rc != 2 || calls != 0 {
		t.Fatalf("the auto-resume at the cap: rc=%d calls=%d, want refused (rc 2) with no cycle run\n%s", rc, calls, stderr)
	}
	if !strings.Contains(stderr, "attempts=3/3 auto-resume=off: the auto-resume budget is spent") {
		t.Fatalf("stderr lacks the refused pause line with the operator action:\n%s", stderr)
	}
	if qp, _ := detectQuotaPause(evolveDir); !strings.Contains(qp.OperatorAction, "evolve loop --resume") {
		t.Fatalf("the checkpoint records operator action %q, want the manual resume", qp.OperatorAction)
	}
}

func TestRunResumeBatch_AnOperatorResumeResetsTheCounterAndRuns(t *testing.T) {
	root, evolveDir := seedQuotaPause(t, checkpoint.DefaultAutoResumeAttempts)

	rc, calls, stderr := resumeOnce(t, root, evolveDir, false)

	if rc != 5 || calls != 1 {
		t.Fatalf("operator resume at the cap: rc=%d calls=%d, want the cycle re-run\n%s", rc, calls, stderr)
	}
	if got := attemptsOnDisk(t, evolveDir); got != 0 {
		t.Fatalf("attempts=%d after the operator resume, want 0", got)
	}
}

func TestRunResumeBatch_ACheckpointThatCannotRecordTheResumeIsAnError(t *testing.T) {
	root, evolveDir := seedQuotaPause(t, 0)
	saved := recordResume
	t.Cleanup(func() { recordResume = saved })
	recordResume = func(string, bool) (checkpoint.Checkpoint, bool, error) {
		return checkpoint.Checkpoint{}, false, errors.New("lock refused")
	}

	rc, calls, stderr := resumeOnce(t, root, evolveDir, true)

	if rc != 2 || calls != 0 || !strings.Contains(stderr, "lock refused") {
		t.Fatalf("rc=%d calls=%d, want rc 2, no cycle run and the cause on stderr\n%s", rc, calls, stderr)
	}
}

func TestParseLoopArgs_AutoResumeIsAResumeTheCounterCounts(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{"--project-root", t.TempDir(), "--auto-resume"}, &stderr)
	if rc != 0 || !cfg.Resume || !cfg.AutoResume {
		t.Fatalf("--auto-resume: rc=%d cfg.Resume=%v cfg.AutoResume=%v, want a counted resume\n%s", rc, cfg.Resume, cfg.AutoResume, stderr.String())
	}
	cfg, rc = parseLoopArgs([]string{"--project-root", t.TempDir(), "--resume"}, &stderr)
	if rc != 0 || !cfg.Resume || cfg.AutoResume {
		t.Fatalf("--resume: rc=%d cfg.Resume=%v cfg.AutoResume=%v, want an operator resume", rc, cfg.Resume, cfg.AutoResume)
	}
}

type cancellingRunner struct{ cancel context.CancelFunc }

func (r cancellingRunner) RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error) {
	return core.CycleResult{}, errors.New("RunCycle is not a resume")
}

func (r cancellingRunner) RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error) {
	r.cancel()
	return core.CycleResult{Cycle: 7}, context.Canceled
}

func (cancellingRunner) SignalSummary() signalcenter.Summary { return signalcenter.Summary{} }

func TestRunResumeBatch_ASignalDuringTheResumedCycleIsASignalStop(t *testing.T) {
	root, evolveDir := seedQuotaPause(t, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errOut bytes.Buffer
	lr := loopResult{}

	rc := runResumeBatch(ctx, loopConfig{ProjectRoot: root, EvolveDir: evolveDir, Resume: true}, cancellingRunner{cancel: cancel}, newFakeLedger(), nil, map[string]string{}, map[string]string{}, &lr, &out, &errOut)

	if rc != 130 || lr.StopReason == "error" {
		t.Fatalf("rc=%d stop=%q, want the signal stop (130), never an error\n%s", rc, lr.StopReason, errOut.String())
	}
}

func TestParseLoopArgs_AReservedWordIsNotAGoal(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer

	_, rc := parseLoopArgs([]string{"--project-root", t.TempDir(), "status"}, &stderr)

	if rc != 10 || !strings.Contains(stderr.String(), "evolve loop status") {
		t.Fatalf("rc=%d stderr=%q, want rc 10 that names `evolve loop status`", rc, stderr.String())
	}
}

package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var errLaneDeadline = errors.New("lane lease deadline")

func cancelledLaneRun(t *testing.T, cycle int) (*cycleRun, *bytes.Buffer, string) {
	t.Helper()
	root, wt := initContinuationRepo(t, cycle)
	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "half_built.go"), []byte("package wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errLaneDeadline)
	var stderr bytes.Buffer
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&stderr))
	cr := &cycleRun{
		o: o, ctx: ctx, cycle: cycle,
		req: CycleRequest{ProjectRoot: root, GoalHash: "g"},
		cs: CycleState{CycleID: cycle, WorkspacePath: ws, ActiveWorktree: wt,
			WorktreeBaseSHA: gitOut(t, wt, "rev-parse", "HEAD"), Phase: "build", ActiveAgent: "build"},
	}
	return cr, &stderr, root
}

func TestAbnormalEpilogue_LaneDeadlineCancelStillSnapshotsAndStamps(t *testing.T) {
	t.Parallel()
	cr, stderr, root := cancelledLaneRun(t, 2008)

	cr.abnormalEpilogue(context.Canceled)

	m, ok, err := continuation.ReadManifest(cr.cs.WorkspacePath)
	if err != nil || !ok {
		t.Fatalf("a lane stopped at its deadline must stamp its continuation: ok=%v err=%v\nstderr:\n%s", ok, err, stderr)
	}
	if m.Cycle != 2008 || m.SnapshotSHA == cr.cs.WorktreeBaseSHA {
		t.Errorf("the stamp must point at a new salvage snapshot of the build work: %+v", m)
	}
	if subj := gitOut(t, cr.cs.ActiveWorktree, "log", "-1", "--format=%s"); subj != salvageSnapshotSubject {
		t.Errorf("the half-built work must be committed as the salvage snapshot, HEAD subject = %q", subj)
	}
	if !strings.Contains(stderr.String(), "cycle 2008 continuation: deadline stamp after the cycle context ended (lane lease deadline)") {
		t.Errorf("each deadline stamp must be logged with its cause, got:\n%s", stderr)
	}
	if cr.cs.Phase != "build" || cr.cs.ActiveAgent != "build" {
		t.Errorf("a cancelled cycle keeps its phase for resume; got phase=%q agent=%q", cr.cs.Phase, cr.cs.ActiveAgent)
	}
	if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-2008.json")); err == nil {
		t.Error("a cancelled cycle must not be sealed by the epilogue dossier; only the stamp runs")
	}
}

func TestAbnormalEpilogue_DeadlineStampStopsAtItsOwnBudget(t *testing.T) {
	cr, stderr, _ := cancelledLaneRun(t, 2010)
	cr.o.stampBudget = 50 * time.Millisecond
	orig := gitRunner
	gitRunner = sysexec.RunFunc(func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		<-ctx.Done()
		return -1, ctx.Err()
	})
	t.Cleanup(func() { gitRunner = orig })

	start := time.Now()
	cr.abnormalEpilogue(context.Canceled)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("the deadline stamp held the lane for %s; its budget is 50ms", elapsed)
	}
	if _, ok, _ := continuation.ReadManifest(cr.cs.WorkspacePath); ok {
		t.Error("a stamp that ran out of budget must not write a manifest")
	}
	if !strings.Contains(stderr.String(), "deadline stamp after the cycle context ended") {
		t.Errorf("the attempt must be logged even when it runs out of budget, got:\n%s", stderr)
	}
}

func TestDeadlineStampBudget_DefaultsBelowTheLaunchWaitDelay(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if got := o.deadlineStampBudget(); got != defaultDeadlineStampBudget || got >= LaneLaunchWaitDelay {
		t.Errorf("default budget = %s, want %s and below the lane launch WaitDelay %s", got, defaultDeadlineStampBudget, LaneLaunchWaitDelay)
	}
}

func TestAbnormalEpilogue_CancelledWithoutWorktreeWritesNothing(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cr, _ := epilogueRun(t, false)
	cr.o.stderr = &stderr
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cr.ctx = ctx

	cr.abnormalEpilogue(context.Canceled)

	if _, ok, _ := continuation.ReadManifest(cr.cs.WorkspacePath); ok {
		t.Error("no active worktree means nothing to salvage")
	}
	if stderr.Len() != 0 {
		t.Errorf("no active worktree must not log a deadline stamp, got %q", stderr.String())
	}
	if cr.cs.Phase != "retro" {
		t.Errorf("phase changed to %q on a cancelled cycle", cr.cs.Phase)
	}
}

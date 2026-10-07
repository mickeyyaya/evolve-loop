//go:build integration

package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type countingDirtySeam struct {
	calls   atomic.Int32
	okFirst atomic.Int32
}

func (s *countingDirtySeam) read(ctx context.Context, repoRoot string) ([]string, error) {
	if s.calls.Add(1) > s.okFirst.Load() {
		return nil, errors.New("git status keeps failing")
	}
	return defaultGitDirtyPaths(ctx, repoRoot)
}

type recordingBuilder struct{ ran atomic.Bool }

func (b *recordingBuilder) Name() string { return string(PhaseBuild) }
func (b *recordingBuilder) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	b.ran.Store(true)
	return PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func remediationWithSnapshotsThatFailAfter(t *testing.T, okFirst int32) (*cycleRun, *dispatchResult, *recordingBuilder, *atomic.Bool) {
	t.Helper()
	repo, wt := realWorktree(t)
	workspace := t.TempDir()
	seam := &countingDirtySeam{}
	seam.okFirst.Store(okFirst)
	builder := &recordingBuilder{}
	runners := buildRunners(nil)
	runners[PhaseBuild] = builder
	var gateReran atomic.Bool
	gate := &tddLeakRunner{name: string(PhaseTDD), onRun: func() { gateReran.Store(true) }}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithGitDirtyPaths(seam.read),
		WithWorkflowConfig(policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{string(PhaseTDD)}}))
	cr := &cycleRun{o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: repo}, cycle: 1811,
		cs:          CycleState{CycleID: 1811, WorkspacePath: workspace, ActiveWorktree: wt},
		retryConfig: o.retryConfig, workflowConfig: o.workflowConfig}
	dr := &dispatchResult{
		resp:          PhaseResponse{Phase: string(PhaseTDD), Verdict: VerdictFAIL, ArtifactsDir: workspace},
		attemptCount:  1,
		phaseWorktree: wt,
		runner:        gate,
		phaseReq:      PhaseRequest{Cycle: 1811, ProjectRoot: repo, Workspace: workspace, Worktree: wt},
	}
	return cr, dr, builder, &gateReran
}

func TestMaybeRemediate_AFixSnapshotFailureVoidsTheRound(t *testing.T) {
	t.Parallel()
	cr, dr, builder, gateReran := remediationWithSnapshotsThatFailAfter(t, 0)

	act, err := cr.maybeRemediate(PhaseTDD, dr)

	if err != nil || act != loopNext {
		t.Fatalf("a failed fix snapshot must leave the gate FAIL standing: act=%v err=%v", act, err)
	}
	if builder.ran.Load() || gateReran.Load() {
		t.Fatalf("the fix ran without a pre-phase snapshot (builder=%v gate rerun=%v)", builder.ran.Load(), gateReran.Load())
	}
	if dr.resp.Verdict != VerdictFAIL || len(cr.result.Remediations) != 1 || !strings.Contains(cr.result.Remediations[0], "builder-failed") {
		t.Fatalf("verdict=%s remediations=%v, want FAIL standing and one builder-failed note", dr.resp.Verdict, cr.result.Remediations)
	}
}

func TestMaybeRemediate_AGateRerunSnapshotFailureVoidsTheRound(t *testing.T) {
	t.Parallel()
	cr, dr, builder, gateReran := remediationWithSnapshotsThatFailAfter(t, 2)

	act, err := cr.maybeRemediate(PhaseTDD, dr)

	if err != nil || act != loopNext {
		t.Fatalf("act=%v err=%v", act, err)
	}
	if !builder.ran.Load() {
		t.Fatal("the fix should have run: its snapshot succeeded")
	}
	if gateReran.Load() {
		t.Fatal("the gate re-ran without a pre-phase snapshot")
	}
	if dr.resp.Verdict != VerdictFAIL {
		t.Fatalf("verdict=%s, the original FAIL must stand", dr.resp.Verdict)
	}
}

func TestSnapshotMainTree_TwoTransientFailuresFitInsideTheRetries(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	seam := &flakyDirtyPaths{}
	seam.failuresLeft.Store(2)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithGitDirtyPaths(seam.read))
	cr := &cycleRun{o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: repo}, cycle: 7,
		cs: CycleState{CycleID: 7, WorkspacePath: t.TempDir(), ActiveWorktree: wt}}

	if _, _, err := cr.snapshotMainTree(PhaseBuild); err != nil {
		t.Fatalf("two transient failures fit inside three attempts: %v", err)
	}
}

func TestSnapshotMainTree_AMalformedGitPointerIsACheckoutNotAnExemption(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("not a gitdir pointer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seam := &flakyDirtyPaths{}
	seam.alwaysFail.Store(true)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithGitDirtyPaths(seam.read))
	cr := &cycleRun{o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: root}, cycle: 7}

	if guard, _, err := cr.snapshotMainTree(PhaseBuild); err == nil {
		t.Fatalf("a .git that exists but cannot be read is a checkout whose snapshot failed, not an exemption (guard=%v)", guard)
	}
}

func TestSnapshotMainTree_ADotGitRemovedMidCycleDoesNotTurnTheGuardOff(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cr := &cycleRun{o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: repo}, cycle: 1811,
		cs: CycleState{CycleID: 1811, WorkspacePath: RunWorkspacePath(repo, 1811), ActiveWorktree: wt}}
	if guard, _, err := cr.snapshotMainTree(PhaseTDD); err != nil || guard == nil {
		t.Fatalf("first snapshot: guard=%v err=%v", guard, err)
	}
	if err := os.Rename(filepath.Join(repo, ".git"), filepath.Join(repo, ".git.moved")); err != nil {
		t.Fatal(err)
	}

	guard, _, err := cr.snapshotMainTree(PhaseBuild)

	if guard == nil && err == nil {
		t.Fatal("a phase that removes <root>/.git must not switch the guard and recovery off for every later phase")
	}
}

func TestRetryingDirtyPaths_StopsWhenTheContextIsCancelled(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	failing := func(context.Context, string) ([]string, error) {
		calls.Add(1)
		return nil, errors.New("git status failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	_, err := retryingDirtyPaths(failing)(ctx, t.TempDir())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled context must end the retries with its error; got %v", err)
	}
	if calls.Load() != 1 || time.Since(started) >= snapshotRetryBackoff {
		t.Fatalf("calls=%d elapsed=%v: the retry slept or read again after cancellation", calls.Load(), time.Since(started))
	}
}

func TestRecoverBeforeReview_APostPhaseGitFailureFailsRecovery(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseBuild)
	const leak = "go/internal/router/leak.go"
	writeTreeFile(t, repo, leak, "package router\n")
	if err := os.WriteFile(filepath.Join(repo, ".git", "index"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := cr.recoverBeforeReview(PhaseBuild, dr)

	if err == nil || !strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("recovery that cannot read the main tree must fail the phase, not report it clean; got %v", err)
	}
	requireTreeFile(t, repo, leak, "package router\n")
}

func TestRunCycle_APostPhaseGuardReadFailureAbortsThePhase(t *testing.T) {
	t.Parallel()
	root := initAuditLeakRepo(t)
	seam := &countingDirtySeam{}
	seam.okFirst.Store(1 << 30)
	st := &phaseHookStorage{fakeStorage: &fakeStorage{}, phase: PhaseTDD, hook: func() {
		seam.okFirst.Store(seam.calls.Load() + 1)
	}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(gitWorktree{}), WithGitDirtyPaths(seam.read))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if !st.fired {
		t.Fatal("the harness never reached tdd")
	}
	if err == nil || !strings.Contains(err.Error(), "tree-diff") || !strings.Contains(err.Error(), "post-phase") {
		t.Fatalf("a guard that cannot read the main tree after the phase must abort it; got %v", err)
	}
}

func TestRunCycle_AGuardReCheckFailureAfterTheBinaryDiscardAbortsThePhase(t *testing.T) {
	t.Parallel()
	root := initAuditLeakRepo(t)
	seam := &countingDirtySeam{}
	seam.okFirst.Store(1 << 30)
	runners := buildRunners(nil)
	runners[PhaseShip] = &tddLeakRunner{name: string(PhaseShip), onRun: func() {
		writeTreeFile(t, root, "go/internal/router/ship-leak.go", "package router\n")
	}}
	st := &phaseHookStorage{fakeStorage: &fakeStorage{}, phase: PhaseShip, hook: func() {
		seam.okFirst.Store(seam.calls.Load() + 2)
	}}
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(gitWorktree{}), WithGitDirtyPaths(seam.read))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if !st.fired {
		t.Fatal("the harness never reached ship")
	}
	if err == nil || !strings.Contains(err.Error(), "post-phase") {
		t.Fatalf("a re-check the guard cannot read must abort, not read as clean; got %v", err)
	}
}

func TestRunCycleFromPhase_ADotGitRemovedBetweenResumedPhasesDoesNotTurnTheGuardOff(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	var tddRan atomic.Bool
	runners := buildRunners(nil)
	runners[PhaseTDD] = &tddLeakRunner{name: string(PhaseTDD), onRun: func() { tddRan.Store(true) }}
	st := &phaseHookStorage{
		fakeStorage: &fakeStorage{
			state:      State{LastCycleNumber: 9},
			cycleState: CycleState{CycleID: 9, WorkspacePath: t.TempDir(), ActiveWorktree: wt},
		},
		phase: PhaseTDD,
		hook: func() {
			if err := os.Rename(filepath.Join(repo, ".git"), filepath.Join(repo, ".git.moved")); err != nil {
				t.Errorf("remove .git: %v", err)
			}
		},
	}
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: repo}, &ResumePoint{Phase: string(PhaseTriage), CycleID: 9})

	if !st.fired {
		t.Fatalf("the resume never reached tdd (err=%v)", err)
	}
	if err == nil || !strings.Contains(err.Error(), "pre-phase main-tree snapshot") {
		t.Fatalf("the resume decided the root was a checkout at its first phase, so a later phase without a readable .git must stop; got %v", err)
	}
	if tddRan.Load() {
		t.Fatal("tdd ran unguarded after .git disappeared mid-resume")
	}
}

func TestRetryingDirtyPaths_NoReaderStaysNoReader(t *testing.T) {
	if got := retryingDirtyPaths(nil); got != nil {
		t.Fatal("a nil reader must stay nil, so the guard's own no-seam rule applies instead of a call through nil")
	}
}

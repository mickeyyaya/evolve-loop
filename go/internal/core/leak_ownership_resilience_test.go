//go:build integration

package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type flakyDirtyPaths struct {
	failuresLeft atomic.Int32
	alwaysFail   atomic.Bool
}

func (f *flakyDirtyPaths) read(ctx context.Context, repoRoot string) ([]string, error) {
	if f.alwaysFail.Load() {
		return nil, errors.New("git status keeps failing")
	}
	if f.failuresLeft.Load() > 0 {
		f.failuresLeft.Add(-1)
		return nil, errors.New("transient git status failure")
	}
	return defaultGitDirtyPaths(ctx, repoRoot)
}

func TestRunCycle_ATransientPrePhaseSnapshotFailureIsRetriedAndTheLeakRecovered(t *testing.T) {
	t.Parallel()
	root := initAuditLeakRepo(t)
	const leak = "go/internal/router/leak.go"
	seam := &flakyDirtyPaths{}
	runners := buildRunners(nil)
	runners[PhaseTDD] = &tddLeakRunner{name: string(PhaseTDD), onRun: func() { writeTreeFile(t, root, leak, "package router\n") }}
	st := &phaseHookStorage{fakeStorage: &fakeStorage{}, phase: PhaseTDD, hook: func() { seam.failuresLeft.Store(1) }}
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(gitWorktree{}), WithGitDirtyPaths(seam.read))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if !st.fired {
		t.Fatal("the harness never reached tdd")
	}
	if err != nil {
		t.Fatalf("a snapshot failure that clears on retry must not fail the cycle: %v", err)
	}
	requireAbsent(t, root, leak)
}

func TestRunCycle_APrePhaseSnapshotThatKeepsFailingStopsBeforeThePhaseRuns(t *testing.T) {
	t.Parallel()
	root := initAuditLeakRepo(t)
	seam := &flakyDirtyPaths{}
	var tddRan atomic.Bool
	runners := buildRunners(nil)
	runners[PhaseTDD] = &tddLeakRunner{name: string(PhaseTDD), onRun: func() { tddRan.Store(true) }}
	st := &phaseHookStorage{fakeStorage: &fakeStorage{}, phase: PhaseTDD, hook: func() { seam.alwaysFail.Store(true) }}
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(gitWorktree{}), WithGitDirtyPaths(seam.read))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if err == nil || !strings.Contains(err.Error(), "pre-phase main-tree snapshot") {
		t.Fatalf("a snapshot that never succeeds must stop the cycle loudly; got: %v", err)
	}
	if tddRan.Load() {
		t.Fatal("tdd ran without a pre-phase snapshot, so a leak it made could not be told from the operator's work")
	}
}

func TestRunCycleFromPhase_APrePhaseSnapshotThatKeepsFailingStopsBeforeThePhaseRuns(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	seam := &flakyDirtyPaths{}
	seam.alwaysFail.Store(true)
	var triageRan atomic.Bool
	runners := buildRunners(nil)
	runners[PhaseTriage] = &tddLeakRunner{name: string(PhaseTriage), onRun: func() { triageRan.Store(true) }}
	st := &fakeStorage{state: State{LastCycleNumber: 9}, cycleState: CycleState{CycleID: 9, WorkspacePath: t.TempDir(), ActiveWorktree: wt}}
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithGitDirtyPaths(seam.read))

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: repo}, &ResumePoint{Phase: string(PhaseTriage), CycleID: 9})

	if err == nil || !strings.Contains(err.Error(), "pre-phase main-tree snapshot") {
		t.Fatalf("a resumed phase without a snapshot must stop loudly; got: %v", err)
	}
	if triageRan.Load() {
		t.Fatal("the resumed phase ran without a pre-phase snapshot")
	}
}

func TestRecoverBeforeReview_AFailedSnapshotNeverRecoversAgainstAnEmptyBaseline(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	writeTreeFile(t, repo, "operator-notes.md", "operator's uncommitted work\n")
	seam := &flakyDirtyPaths{}
	seam.alwaysFail.Store(true)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithGitDirtyPaths(seam.read))
	cr := &cycleRun{o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: repo}, cycle: 7,
		cs: CycleState{CycleID: 7, WorkspacePath: t.TempDir(), ActiveWorktree: wt}}
	if _, before, err := cr.snapshotMainTree(PhaseBuild); err == nil {
		t.Fatalf("a seam that keeps failing must be reported, not read as a snapshot of %v", before)
	}
	dr := &dispatchResult{phaseWorktree: wt}

	if err := cr.recoverBeforeReview(PhaseBuild, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireTreeFile(t, repo, "operator-notes.md", "operator's uncommitted work\n")
	requireAbsent(t, wt, "operator-notes.md")
}

func TestRecoverBeforeReview_SiblingsStagedChangesToItsCommittedEvalAreLeft(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		stage func(t *testing.T, repo string, sibling fleetLane)
		want  string
	}{
		{"staged deletion", func(t *testing.T, repo string, s fleetLane) { gitInRepo(t, repo, "rm", "-q", s.evalPath()) }, "D"},
		{"staged add", func(t *testing.T, repo string, s fleetLane) {
			gitInRepo(t, repo, "rm", "-q", "--cached", s.evalPath())
			gitInRepo(t, repo, "commit", "-q", "-m", "drop it from HEAD")
			writeTreeFile(t, repo, s.evalPath(), "sibling staged add\n")
			gitInRepo(t, repo, "add", s.evalPath())
		}, "A"},
		{"staged edit", func(t *testing.T, repo string, s fleetLane) {
			writeTreeFile(t, repo, s.evalPath(), "sibling staged edit\n")
			gitInRepo(t, repo, "add", s.evalPath())
		}, "M"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, lane, sibling := twoLanesOnOneMainTree(t)
			writeTreeFile(t, repo, sibling.evalPath(), "committed by an earlier cycle\n")
			gitInRepo(t, repo, "add", "-f", sibling.evalPath())
			gitInRepo(t, repo, "commit", "-q", "-m", "earlier cycle eval")
			gitInRepo(t, lane.worktree, "checkout", "-q", "--detach", gitInRepo(t, repo, "rev-parse", "HEAD"))
			cr, dr := lane.phaseStart(t, repo, 1811, PhaseBuild)
			tc.stage(t, repo, sibling)

			if err := cr.recoverBeforeReview(PhaseBuild, dr); err != nil {
				t.Fatalf("recovery failed: %v", err)
			}

			if st := porcelainStatusOf(t, repo, sibling.evalPath()); st != tc.want {
				t.Fatalf("the sibling's %s of its committed eval was undone by this lane's recovery; main index status=%q, want %q", tc.name, st, tc.want)
			}
		})
	}
}

func TestMaybeRemediate_TheGateReRunsOwnLeakIsRecovered(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	workspace := t.TempDir()
	const leak = "go/internal/router/gate-rerun-leak.go"
	runners := buildRunners(nil)
	runners[PhaseBuild] = remediationNoopBuilder{}
	gate := &tddLeakRunner{name: string(PhaseTDD), onRun: func() { writeTreeFile(t, repo, leak, "package router\n") }}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorkflowConfig(policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{string(PhaseTDD)}}))
	cr := &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: repo}, cycle: 1811,
		cs:          CycleState{CycleID: 1811, WorkspacePath: workspace, ActiveWorktree: wt},
		retryConfig: o.retryConfig, workflowConfig: o.workflowConfig,
	}
	dr := dispatchResult{
		resp:          PhaseResponse{Phase: string(PhaseTDD), Verdict: VerdictFAIL, ArtifactsDir: workspace},
		attemptCount:  1,
		phaseWorktree: wt,
		runner:        gate,
		phaseReq:      PhaseRequest{Cycle: 1811, ProjectRoot: repo, Workspace: workspace, Worktree: wt},
	}
	takePhaseSnapshot(t, cr, &dr, PhaseTDD)

	_, err := cr.maybeRemediate(PhaseTDD, &dr)

	if err != nil && !strings.Contains(err.Error(), "tree-diff") {
		t.Fatalf("remediation failed: %v", err)
	}
	requireAbsent(t, repo, leak)
}

func TestRecoverBeforeReview_ADifferingMainTreeCopyIsQuarantinedNotDeleted(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseBuild)
	writeTreeFile(t, lane.worktree, lane.evalPath(), "fuller TDD eval\n")
	writeTreeFile(t, repo, lane.evalPath(), "a newer edit made in the main tree\n")

	if err := cr.recoverBeforeReview(PhaseBuild, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireTreeFile(t, lane.worktree, lane.evalPath(), "fuller TDD eval\n")
	requireAbsent(t, repo, lane.evalPath())
	requireTreeFile(t, quarantineDir(repo, 1811), lane.evalPath(), "a newer edit made in the main tree\n")
}

func TestRecoverBeforeReview_AnIdenticalMainTreeCopyIsDroppedWithoutQuarantine(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseBuild)
	writeTreeFile(t, lane.worktree, lane.evalPath(), "the same eval\n")
	writeTreeFile(t, repo, lane.evalPath(), "the same eval\n")

	if err := cr.recoverBeforeReview(PhaseBuild, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireTreeFile(t, lane.worktree, lane.evalPath(), "the same eval\n")
	requireAbsent(t, repo, lane.evalPath())
	if _, err := os.Stat(quarantineDir(repo, 1811)); !os.IsNotExist(err) {
		t.Fatalf("identical bytes lose nothing, so nothing is quarantined (stat err=%v)", err)
	}
}

func TestRecoverBeforeReview_AKeyedPathNoLiveLaneHoldsFailsRecovery(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseTDD)
	const retired = ".evolve/evals/retired-item.md"
	writeTreeFile(t, repo, retired, "no live lane holds this item\n")

	err := cr.recoverBeforeReview(PhaseTDD, dr)

	if err == nil || !strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("a keyed path that is not this lane's and no live lane holds must fail recovery; got: %v", err)
	}
	requireTreeFile(t, repo, retired, "no live lane holds this item\n")
	requireAbsent(t, lane.worktree, retired)
}

func TestRunCycleFromPhase_AnEditToAnUnownedCommittedEvalFailsTheResume(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	workspace := t.TempDir()
	writeLanePin(t, workspace, "own-slug")
	const other = ".evolve/evals/committed-other.md"
	writeTreeFile(t, repo, other, "committed\n")
	gitInRepo(t, repo, "add", "-A")
	gitInRepo(t, repo, "commit", "-q", "-m", "eval")
	gitInRepo(t, wt, "checkout", "-q", "--detach", gitInRepo(t, repo, "rev-parse", "HEAD"))
	var ran atomic.Bool
	runners := buildRunners(nil)
	runners[PhaseTDD] = &tddLeakRunner{name: string(PhaseTDD), onRun: func() {
		writeTreeFile(t, repo, other, "this lane's own edit\n")
		ran.Store(true)
	}}
	st := &fakeStorage{state: State{LastCycleNumber: 9}, cycleState: CycleState{CycleID: 9, WorkspacePath: workspace, ActiveWorktree: wt}}
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: repo}, &ResumePoint{Phase: string(PhaseTriage), CycleID: 9})

	if !ran.Load() {
		t.Fatalf("the resumed tdd never ran (err=%v)", err)
	}
	if err == nil || !strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("the resume path has no tree-diff guard, so recovery itself must fail closed; got: %v", err)
	}
	if body, _ := os.ReadFile(filepath.Join(wt, other)); string(body) == "this lane's own edit\n" {
		t.Fatal("the unowned edit was moved into the lane's worktree")
	}
}

func porcelainStatusOf(t *testing.T, repo, rel string) string {
	t.Helper()
	for _, line := range strings.Split(gitInRepo(t, repo, "status", "--porcelain", "-uall"), "\n") {
		if len(line) > 3 && porcelainPath(line) == rel {
			return strings.TrimSpace(line[:2])
		}
	}
	return ""
}

func TestRecoverBeforeReview_AnExpiredSiblingLeaseDoesNotHoldItsEval(t *testing.T) {
	t.Parallel()
	repo, lane, sibling := twoLanesOnOneMainTree(t)
	writeRunLease(t, sibling.workspace, time.Now().Add(-6*time.Hour))
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseTDD)
	writeTreeFile(t, repo, sibling.evalPath(), "left by a lane whose lease expired\n")

	err := cr.recoverBeforeReview(PhaseTDD, dr)

	if err == nil || !strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("a dead lane's eval must fail recovery, not be left as if held; got %v", err)
	}
}

func TestRecoverBeforeReview_AnUnheldPathAfterAHeldOneStillFailsRecovery(t *testing.T) {
	t.Parallel()
	repo, lane, sibling := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseTDD)
	writeTreeFile(t, repo, sibling.evalPath(), "live sibling's eval\n")
	writeTreeFile(t, repo, ".evolve/evals/retired-item.md", "no live lane holds this\n")

	err := cr.recoverBeforeReview(PhaseTDD, dr)

	if err == nil || !strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("each keyed path needs its own live holder; got %v", err)
	}
}

func TestRecoverBeforeReview_ADifferingCopyOfTheSameLengthIsQuarantined(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseBuild)
	writeTreeFile(t, lane.worktree, lane.evalPath(), "aaaa\n")
	writeTreeFile(t, repo, lane.evalPath(), "bbbb\n")

	if err := cr.recoverBeforeReview(PhaseBuild, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireTreeFile(t, quarantineDir(repo, 1811), lane.evalPath(), "bbbb\n")
}

func TestRecoverBeforeReview_AnUnreadableWorktreeCopyKeepsTheMainCopy(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseBuild)
	writeTreeFile(t, lane.worktree, lane.evalPath(), "worktree copy\n")
	writeTreeFile(t, repo, lane.evalPath(), "main copy\n")
	wtCopy := filepath.Join(lane.worktree, lane.evalPath())
	if err := os.Chmod(wtCopy, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(wtCopy, 0o644) })

	err := cr.recoverBeforeReview(PhaseBuild, dr)

	if err == nil {
		t.Fatal("a compare that cannot read both copies must fail recovery")
	}
	requireTreeFile(t, repo, lane.evalPath(), "main copy\n")
}

func TestRecoverBeforeReview_QuarantiningTheSamePathTwiceKeepsBothCopies(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	writeTreeFile(t, lane.worktree, lane.evalPath(), "worktree eval\n")
	first, firstDispatch := lane.phaseStart(t, repo, 1811, PhaseTDD)
	writeTreeFile(t, repo, lane.evalPath(), "first main-tree edit\n")
	if err := first.recoverBeforeReview(PhaseTDD, firstDispatch); err != nil {
		t.Fatalf("first recovery: %v", err)
	}
	second, secondDispatch := lane.phaseStart(t, repo, 1811, PhaseBuild)
	writeTreeFile(t, repo, lane.evalPath(), "second main-tree edit\n")

	if err := second.recoverBeforeReview(PhaseBuild, secondDispatch); err != nil {
		t.Fatalf("second recovery: %v", err)
	}

	quarantined := filepath.Join(quarantineDir(repo, 1811), filepath.FromSlash(lane.evalPath()))
	requireTreeFile(t, filepath.Dir(quarantined), filepath.Base(quarantined), "first main-tree edit\n")
	requireTreeFile(t, filepath.Dir(quarantined), filepath.Base(quarantined)+".1", "second main-tree edit\n")
}

func TestRecoverBeforeReview_AConsoleLeasedPathIsLeftJustAsTheGuardWaivesIt(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseTDD)
	const leasedPath = "go/acs/cycle1700/predicates_test.go"
	cr.consoleLeased = map[string]bool{leasedPath: true}
	writeTreeFile(t, repo, leasedPath, "operator fixing an old predicate under a console lease\n")

	real, waived := filterRealLeaks([]string{leasedPath}, cr.leakExemptions(), io.Discard)
	err := cr.recoverBeforeReview(PhaseTDD, dr)

	if len(real) != 0 || waived != 1 {
		t.Fatalf("the guard must waive the leased path: real=%v waived=%d", real, waived)
	}
	if err != nil {
		t.Fatalf("recovery must leave a leased path the guard waives, not fail the phase: %v", err)
	}
	requireTreeFile(t, repo, leasedPath, "operator fixing an old predicate under a console lease\n")
	requireAbsent(t, lane.worktree, leasedPath)
}

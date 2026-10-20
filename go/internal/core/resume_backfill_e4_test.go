package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func writeUnusableRunStatesE4(t *testing.T, runs string) {
	t.Helper()
	writeFile(t, filepath.Join(runs, "stray.txt"), "not a run dir")
	if err := os.MkdirAll(filepath.Join(runs, "cycle-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(runs, "cycle-2", CycleStateFile), "{not json")
	writeStateFile(t, filepath.Join(runs, "cycle-3"), map[string]any{"cycle_id": 3})
	writeStateFile(t, filepath.Join(runs, "cycle-4"), map[string]any{"cycle_id": 4, "checkpoint": map[string]any{"enabled": false, "reason": "quota-likely"}})
}

func TestDiscoverPerRunResumeState_SkipsEveryRunWithoutALiveCheckpoint(t *testing.T) {
	t.Parallel()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	writeUnusableRunStatesE4(t, filepath.Join(evolveDir, "runs"))

	rp, err := discoverPerRunResumeState(evolveDir, t.TempDir(), ResumeOptions{}.withDefaults())

	if !errors.Is(err, errNoPerRunCandidates) || rp != nil {
		t.Fatalf("discover = (%+v, %v), want no candidates from unusable runs", rp, err)
	}
}

func TestDiscoverPerRunResumeState_ATiedSaveTimePrefersTheNewerCycle(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	runs := filepath.Join(evolveDir, "runs")
	writeUnusableRunStatesE4(t, runs)
	writeStateFile(t, filepath.Join(runs, "cycle-10"), fleetCheckpointState(10, "2026-10-12T05:00:00Z", "audit", wt))
	writeStateFile(t, filepath.Join(runs, "cycle-11"), fleetCheckpointState(11, "2026-10-12T05:00:00Z", "tdd", wt))
	writeStateFile(t, filepath.Join(runs, "cycle-9"), fleetCheckpointState(9, "2026-10-12T04:59:59Z", "build", wt))

	rp, err := discoverPerRunResumeState(evolveDir, tmp, ResumeOptions{}.withDefaults())

	if err != nil || rp.CycleID != 11 || rp.Phase != "tdd" {
		t.Fatalf("discover = (%+v, %v), want cycle 11 to win the tie on save time", rp, err)
	}
}

func TestRunCycleFromPhase_AnUnsafeConfigOrAHeldLockRefusesBeforeAnyPhase(t *testing.T) {
	t.Parallel()
	runners := buildRunners(nil)
	unsafe := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	unsafe.safetyViolations = []string{"bypass without operator"}
	_, err := unsafe.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 3})
	if !errors.Is(err, ErrUnsafeConfig) || !strings.Contains(err.Error(), "bypass without operator") {
		t.Fatalf("unsafe config err = %v", err)
	}

	locked := NewOrchestrator(&fakeStorage{lockErr: errors.New("held by pid 77")}, &fakeLedger{}, runners)
	_, err = locked.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 3})
	if err == nil || err.Error() != "acquire lock: held by pid 77" {
		t.Fatalf("held lock err = %v", err)
	}
	if runners[PhaseBuild].(*fakeRunner).calls != 0 {
		t.Fatal("no phase may run after a refused resume")
	}
}

type scriptedResumeRunnerE4 struct {
	calls int
	resp  PhaseResponse
	err   error
	after func()
}

func (r *scriptedResumeRunnerE4) Name() string { return string(PhaseBuild) }
func (r *scriptedResumeRunnerE4) Run(context.Context, PhaseRequest) (PhaseResponse, error) {
	r.calls++
	if r.after != nil {
		r.after()
	}
	return r.resp, r.err
}

func TestReviewResumedDeliverable_NoReviewerOrASkippedPhaseIsReturnedUnreviewed(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.reviewer = nil
	resp := PhaseResponse{Phase: "build", Verdict: VerdictFAIL, ArtifactsDir: "/ws"}
	got, err := o.reviewResumedDeliverable(context.Background(), t.TempDir(), 4, CycleState{CycleID: 4}, PhaseBuild, &scriptedResumeRunnerE4{}, PhaseRequest{}, resp, nil, (&cycleRun{}).countDispatch)
	if err != nil || !reflect.DeepEqual(got, resp) {
		t.Fatalf("no reviewer = (%+v, %v), want the response unchanged", got, err)
	}

	rev := &recordingReviewer{default_: ReviewResult{Approve: false, Reason: "never asked"}}
	o.reviewer = rev
	skipped := PhaseResponse{Phase: "build", Verdict: VerdictSKIPPED}
	got, err = o.reviewResumedDeliverable(context.Background(), t.TempDir(), 4, CycleState{CycleID: 4}, PhaseBuild, &scriptedResumeRunnerE4{}, PhaseRequest{}, skipped, nil, (&cycleRun{}).countDispatch)
	if err != nil || !reflect.DeepEqual(got, skipped) || len(rev.calls) != 0 {
		t.Fatalf("skipped = (%+v, %v) after %d reviews, want it returned unreviewed", got, err, len(rev.calls))
	}
}

func TestReviewResumedDeliverable_ACorrectionThatCannotBeEvaluatedStopsTheResume(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		runner  *scriptedResumeRunnerE4
		wantErr string
	}{
		{"a dispatch failure", &scriptedResumeRunnerE4{err: errors.New("pane died")}, `resume review gate: phase "build" correction 1 dispatch failed: pane died`},
		{"a non-canonical verdict", &scriptedResumeRunnerE4{resp: PhaseResponse{Verdict: "MAYBE"}}, `resume review gate: phase "build" correction 1 returned non-canonical verdict "MAYBE"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rev := &recordingReviewer{decide: map[string]ReviewResult{"build": {Approve: false, Reason: "missing header"}}}
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
			o.reviewer = rev

			_, err := o.reviewResumedDeliverable(context.Background(), t.TempDir(), 4, CycleState{CycleID: 4, WorkspacePath: t.TempDir()}, PhaseBuild, tc.runner, PhaseRequest{}, PhaseResponse{Phase: "build", Verdict: VerdictPASS}, nil, (&cycleRun{}).countDispatch)

			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if tc.runner.calls != 1 || len(rev.calls) != 1 {
				t.Fatalf("runner=%d reviews=%d: the unevaluable correction must not be re-reviewed", tc.runner.calls, len(rev.calls))
			}
		})
	}
}

func TestReviewResumedDeliverable_ALeakWrittenByTheCorrectionStopsBeforeTheReReview(t *testing.T) {
	t.Parallel()

	root := gittest.Fixture(t).Dir
	blocked := filepath.Join(root, "blocked-worktree")
	writeFile(t, blocked, "not a directory\n")
	runner := &scriptedResumeRunnerE4{resp: PhaseResponse{Phase: "build", Verdict: VerdictPASS}, after: func() {
		writeFile(t, filepath.Join(root, "leak.txt"), "escaped correction output\n")
	}}
	rev := &recordingReviewer{decide: map[string]ReviewResult{"build": {Approve: false, Reason: "missing header"}}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.reviewer = rev
	cs := CycleState{CycleID: 9, RunID: "run-9", ActiveWorktree: blocked, WorkspacePath: t.TempDir()}

	var err error
	var sink injectedStderr
	o.stderr = &sink
	_, err = o.reviewResumedDeliverable(context.Background(), root, 9, cs, PhaseBuild, runner, PhaseRequest{}, PhaseResponse{Phase: "build", Verdict: VerdictPASS}, map[string]bool{"blocked-worktree": true}, (&cycleRun{}).countDispatch)

	if err == nil || !strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("err = %v, want the correction's leak to stop the resume", err)
	}
	if runner.calls != 1 || len(rev.calls) != 1 {
		t.Fatalf("runner=%d reviews=%d: the leaked correction must never be re-reviewed", runner.calls, len(rev.calls))
	}
}

package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type claimingBuild struct {
	defects    []string
	dispatches *int
}

func (claimingBuild) Name() string { return string(PhaseBuild) }

func (b claimingBuild) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	*b.dispatches++
	sentinel := phasecontract.RenderVerdictSentinelWithFailure("build", VerdictFAIL, &phasecontract.FailureBlock{Class: "code-build-fail", Defects: b.defects})
	if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), []byte("## Changes\n- tests\n"+sentinel+"\n"), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace}, nil
}

type floorRejecting struct{ failures []string }

func (r floorRejecting) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) {
		return ReviewResult{Approve: true}
	}
	return ReviewResult{Approve: false, Retry: true, Reason: "build handoff floor: " + strings.Join(r.failures, "\n"), Failures: r.failures}
}

type blockerRun struct {
	result     CycleResult
	err        error
	dispatches int
	workspace  string
	worktree   *fakeWorktree
	signals    []signalcenter.Event
}

func runBlockerCycle(t *testing.T, failures, defects []string, member func(string) bool, decision string, env map[string]string) blockerRun {
	t.Helper()
	root := writeClaimableInbox(t, 1)
	repo := gittest.Fixture(t)
	for _, rel := range []string{selfcheckPath, breakerPath} {
		writeBlockerFile(t, repo.Dir, rel, "package core\n")
	}
	writeBlockerFile(t, repo.Dir, "go/acs/cycle10/predicates_test.go", strings.ReplaceAll(cycle1950Predicates, "cycle1950", "cycle10"))
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	run := blockerRun{worktree: &fakeWorktree{path: repo.Dir}}
	runners := buildRunners(nil)
	runners[PhaseTriage] = triageDecisionRunner{verdict: VerdictPASS, decision: decision}
	runners[PhaseBuild] = claimingBuild{defects: defects, dispatches: &run.dispatches}
	center := signalcenter.New()
	var mu sync.Mutex
	center.Subscribe(func(e signalcenter.Event) {
		mu.Lock()
		defer mu.Unlock()
		run.signals = append(run.signals, e)
	})
	opts := []Option{WithWorktreeProvisioner(run.worktree), WithReviewer(floorRejecting{failures: failures}), WithSignalCenter(center)}
	if member != nil {
		opts = append(opts, WithProtectedSurface(member))
	}
	run.result, run.err = NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 9}}, &fakeLedger{}, runners, opts...).RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "lane-goal", Env: env,
	})
	center.Flush()
	run.workspace = filepath.Join(root, ".evolve", "runs", "cycle-10")
	return run
}

func (r blockerRun) signalCodes() []string {
	var codes []string
	for _, e := range r.signals {
		codes = append(codes, string(e.Code))
	}
	return codes
}

func TestBuildBlockerExit_AProvenBlockerEndsTheRoundsAndRoutesTheItemToTheConsole(t *testing.T) {
	t.Parallel()
	run := runBlockerCycle(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, protectsSelfcheck, oneCardDecision, laneEnv)

	if run.err != nil {
		t.Fatalf("RunCycle: %v; a proven blocker is a planned ending, not an abort", run.err)
	}
	if run.dispatches != 1 {
		t.Errorf("build dispatched %d times; a proven blocker spends no correction round", run.dispatches)
	}
	if run.result.TerminationReason != CycleTerminationBlockedNeedsConsole || run.result.FinalVerdict != VerdictFAIL {
		t.Errorf("result = %+v; want FAIL with reason %q", run.result, CycleTerminationBlockedNeedsConsole)
	}
	if len(run.worktree.cleaned) != 0 {
		t.Errorf("the salvage worktree must be kept: cleaned=%v", run.worktree.cleaned)
	}
	entries, err := phasetiming.Read(run.workspace)
	if err != nil || len(entries) == 0 {
		t.Fatalf("phase timing: %v", err)
	}
	last := entries[len(entries)-1]
	codes := cyclestate.ErrorCodes(last.Diagnostics)
	if last.Phase != string(PhaseBuild) || len(codes) == 0 || codes[0] != cyclestate.DiagCodeBlockerRoute || last.Diagnostics[0].Subject != laneScopedItem {
		t.Fatalf("last timing entry = %+v", last)
	}
	if !strings.Contains(last.Diagnostics[0].Message, selfcheckPath) || !strings.Contains(last.Diagnostics[0].Message, "needs a seam") {
		t.Errorf("the route carries the proof and the lane's diagnosis: %q", last.Diagnostics[0].Message)
	}
	if _, err := os.Stat(filepath.Join(run.workspace, "failure-digest.json")); err == nil {
		t.Error("a proven blocker writes no failure digest, so the batch breaker never counts it as a consecutive failure")
	}
	if !strings.Contains(strings.Join(run.signalCodes(), ","), string(CodeBlockerRouted)) {
		t.Errorf("signals = %v; want %s", run.signalCodes(), CodeBlockerRouted)
	}
}

func TestBuildBlockerExit_AClaimedBlockerThatIsNotRealStillFailsAfterTheRounds(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	run := runBlockerCycle(t, []string{cycle1950Failure}, []string{fake}, protectsSelfcheck, oneCardDecision, laneEnv)

	if run.err == nil || run.result.TerminationReason == CycleTerminationBlockedNeedsConsole {
		t.Fatalf("result = %+v err=%v; a claim the floor cannot prove must not escape the failure", run.result, run.err)
	}
	if run.dispatches < 2 {
		t.Errorf("build dispatched %d times; a refused claim keeps the correction rounds", run.dispatches)
	}
	if !strings.Contains(strings.Join(run.signalCodes(), ","), string(CodeBlockerRefused)) {
		t.Errorf("signals = %v; want %s so the refused claim is visible", run.signalCodes(), CodeBlockerRefused)
	}
}

func TestBuildBlockerExit_NoRouteWithoutOneCommittedItem(t *testing.T) {
	t.Parallel()
	twoCards := `{"top_n":[{"id":"claimable-lane-item"},{"id":"other-item"}]}`
	run := runBlockerCycle(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, protectsSelfcheck, twoCards, nil)

	if run.result.TerminationReason == CycleTerminationBlockedNeedsConsole {
		t.Errorf("result = %+v; the console route names exactly one item", run.result)
	}
}

func TestWorktreeDiffSince_NamesChangedPathsAndTestsThatLostLines(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	writeBlockerFile(t, repo.Dir, "go/p/a_test.go", "package p\n\nfunc A() {}\n\nfunc B() {}\n")
	writeBlockerFile(t, repo.Dir, "go/p/b_test.go", "package p\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	writeBlockerFile(t, repo.Dir, "go/p/a_test.go", "package p\n\nfunc A() {}\n")
	writeBlockerFile(t, repo.Dir, "go/p/b_test.go", "package p\n\nfunc C() {}\n")
	writeBlockerFile(t, repo.Dir, "go/p/new.go", "package p\n")

	d, fault := worktreeDiffSince(context.Background(), repo.Dir, "HEAD")

	if fault != "" || strings.Join(d.shrunkTests, ",") != "go/p/a_test.go" {
		t.Errorf("diff = %+v fault=%q; only a_test.go lost lines", d, fault)
	}
	if strings.Join(d.changed, ",") != "go/p/a_test.go,go/p/b_test.go,go/p/new.go" {
		t.Errorf("changed = %v", d.changed)
	}
}

func TestWorktreeDiffSince_AGitFaultIsAnEvidenceFaultNotAnEmptyDiff(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	if _, fault := worktreeDiffSince(context.Background(), t.TempDir(), "HEAD"); fault == "" {
		t.Error("a tree that is not a repository must not read as an empty diff")
	}
	gittest.FaultOn(t, "ls-files")
	if _, fault := worktreeDiffSince(context.Background(), repo.Dir, "HEAD"); !strings.Contains(fault, "ls-files") {
		t.Errorf("fault = %q", fault)
	}
}

func TestIsProvenBlockerResult_OnlyTheBlockedFailEnding(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		result CycleResult
		want   bool
	}{
		"the blocked ending":            {CycleResult{FinalVerdict: VerdictFAIL, TerminationReason: CycleTerminationBlockedNeedsConsole}, true},
		"a code FAIL":                   {CycleResult{FinalVerdict: VerdictFAIL}, false},
		"the TDD protected route":       {CycleResult{FinalVerdict: VerdictFAIL, TerminationReason: CycleTerminationTddProtectedRoute}, false},
		"a PASS with the reason forged": {CycleResult{FinalVerdict: VerdictPASS, TerminationReason: CycleTerminationBlockedNeedsConsole}, false},
	} {
		if got := IsProvenBlockerResult(tc.result); got != tc.want {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

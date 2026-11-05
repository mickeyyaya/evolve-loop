package core

import (
	"context"
	"errors"
	"maps"
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
	rounds     [][]string
	dispatches *int
}

func (claimingBuild) Name() string { return string(PhaseBuild) }

func (b claimingBuild) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	*b.dispatches++
	defects := b.defects
	if len(b.rounds) > 0 {
		defects = b.rounds[min(*b.dispatches, len(b.rounds))-1]
	}
	sentinel := phasecontract.RenderVerdictSentinelWithFailure("build", VerdictFAIL, &phasecontract.FailureBlock{Class: "code-build-fail", Defects: defects})
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
	directives []string
}

func runBlockerCycle(t *testing.T, failures, defects []string, member func(string) bool, decision string, env map[string]string) blockerRun {
	t.Helper()
	return runBlockerCycleWith(t, failures, defects, member, decision, env, nil)
}

type blockerExtra func(t *testing.T, root string) []Option

func runBlockerCycleWith(t *testing.T, failures, defects []string, member func(string) bool, decision string, env map[string]string, tune func(map[Phase]PhaseRunner), extras ...blockerExtra) blockerRun {
	t.Helper()
	return runBlockerRounds(t, failures, claimingBuild{defects: defects}, member, decision, env, tune, extras...)
}

func runBlockerRounds(t *testing.T, failures []string, build claimingBuild, member func(string) bool, decision string, env map[string]string, tune func(map[Phase]PhaseRunner), extras ...blockerExtra) blockerRun {
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
	if tune != nil {
		tune(runners)
	}
	spy := &resumedBuildRun{}
	runners[PhaseBuild] = directiveSpy{inner: claimingBuild{defects: build.defects, rounds: build.rounds, dispatches: &run.dispatches}, run: spy}
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
	for _, extra := range extras {
		opts = append(opts, extra(t, root)...)
	}
	run.result, run.err = NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 9}}, &fakeLedger{}, runners, opts...).RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "lane-goal", Env: env,
	})
	center.Flush()
	run.directives = spy.directives
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

func (r blockerRun) reasonOf(code signalcenter.Code) (string, bool) {
	for _, e := range r.signals {
		if e.Code == code {
			return e.Reason, true
		}
	}
	return "", false
}

func TestBuildBlockerExit_TheFreshSignalsCarryTheExactText(t *testing.T) {
	t.Parallel()
	routed := runBlockerCycle(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, protectsSelfcheck, oneCardDecision, laneEnv)
	wantRouted := "blocked: needs console — phase build stopped on a proven blocker the lane cannot remove: protected-path: the fix needs " + selfcheckPath + ", a protected control-plane path (the lane said: " + cycle1946Claim + ")"
	if got, ok := routed.reasonOf(CodeBlockerRouted); !ok || got != wantRouted {
		t.Errorf("routed reason = %q ok=%v\nwant %q", got, ok, wantRouted)
	}

	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	refused := runBlockerCycle(t, []string{cycle1950Failure}, []string{fake}, protectsSelfcheck, oneCardDecision, laneEnv)
	wantRefused := "the declared blocker is not proven: \"" + fake + "\": " + breakerPath + " is not on the protected-surface manifest, so the lane can edit it. Next: correct the BLOCKER line of your report so each claim is true, or fix the failures without declaring a blocker."
	if got, ok := refused.reasonOf(CodeBlockerRefused); !ok || got != wantRefused {
		t.Errorf("refused reason = %q ok=%v\nwant %q", got, ok, wantRefused)
	}
	if len(refused.directives) < 2 || !strings.Contains(refused.directives[1], "The host refused your BLOCKER claim: "+wantRefused) {
		t.Errorf("directives = %q; the fresh correction tells the lane why the host refused the claim", refused.directives)
	}
}

func TestBuildBlockerExit_NoRouteTextNamesTheCauseAndTheNextStep(t *testing.T) {
	t.Parallel()
	twoCards := `{"top_n":[{"id":"claimable-lane-item"},{"id":"other-item"}]}`
	run := runBlockerCycle(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, protectsSelfcheck, twoCards, nil)
	want := "the blocker is proven, and the triage decision commits items claimable-lane-item, other-item, and a console route needs exactly one, so this cycle ends with no route. Next: at the boundary, the console routes each item that needs this path with evolve inbox route-console <id> <reason> <cycle>. A quarantined item needs evolve inbox quarantine release <id> first."
	if got, ok := run.reasonOf(CodeBlockerRefused); !ok || got != want {
		t.Errorf("reason = %q ok=%v\nwant %q", got, ok, want)
	}
}

func TestBuildBlockerExit_TheFreshBuildEntrySealsThePredicateBase(t *testing.T) {
	t.Parallel()
	run := runBlockerCycle(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, protectsSelfcheck, oneCardDecision, laneEnv)
	got, _ := readPredicateSnapshot(run.workspace)
	want := cyclePredicateSources(run.worktree.path, 10)
	if len(want) == 0 || !maps.Equal(got, want) {
		t.Errorf("sealed base = %v\nwant the predicate sources of cycle 10 %v", got, want)
	}
}

func TestBuildBlockerExit_AnUnroutableBlockerEndsAfterTheFirstDispatchWithNoDirective(t *testing.T) {
	t.Parallel()
	for _, tc := range blockerEndCases {
		if tc.outcome != Unroutable || !tc.fresh {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			run := runBlockerCycle(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, protectsSelfcheck, tc.decision, tc.env)

			var ended blockerEnd
			if !errors.As(run.err, &ended) || ended.outcome != Unroutable {
				t.Fatalf("err = %v; want the typed end of an unroutable blocker", run.err)
			}
			if run.dispatches != 1 {
				t.Errorf("build dispatched %d times; the end spends no correction round", run.dispatches)
			}
			if len(run.directives) != 1 || run.directives[0] != "" {
				t.Errorf("directives = %q; no directive follows the first dispatch", run.directives)
			}
			if run.result.TerminationReason == CycleTerminationBlockedNeedsConsole {
				t.Errorf("result = %+v; only a routable blocker routes", run.result)
			}
			entries, err := phasetiming.Read(run.workspace)
			if err != nil || len(entries) == 0 {
				t.Fatalf("phase timing: %v", err)
			}
			last := entries[len(entries)-1]
			for i := len(entries) - 1; i >= 0 && last.Phase == string(PhaseRetro); i-- {
				last = entries[i]
			}
			if codes := cyclestate.ErrorCodes(last.Diagnostics); last.Verdict != string(VerdictFAIL) || len(codes) == 0 || codes[0] != cyclestate.DiagCodeBlockerUnroutable {
				t.Errorf("ending entry = %+v; want the FAIL outcome to lead with %s", last, cyclestate.DiagCodeBlockerUnroutable)
			}
		})
	}
}

type sealBlockingRunner struct{ inner PhaseRunner }

func (r sealBlockingRunner) Name() string { return r.inner.Name() }

func (r sealBlockingRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.MkdirAll(filepath.Join(req.Workspace, predicateSnapshotFile+".tmp", "held"), 0o755); err != nil {
		return PhaseResponse{}, err
	}
	return r.inner.Run(ctx, req)
}

func TestBuildBlockerExit_ASealFailureAbortsBeforeAnyDispatch(t *testing.T) {
	t.Parallel()
	run := runBlockerCycleWith(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, protectsSelfcheck, oneCardDecision, laneEnv, func(runners map[Phase]PhaseRunner) {
		runners[PhaseTDD] = sealBlockingRunner{inner: runners[PhaseTDD]}
	})
	if run.err == nil || !strings.Contains(run.err.Error(), "seal predicate baseline") {
		t.Fatalf("err = %v; a base that cannot be stored stops the cycle", run.err)
	}
	if run.dispatches != 0 {
		t.Errorf("build dispatched %d times; the seal precedes the dispatch", run.dispatches)
	}
}

func TestProvenBlocker_AReviewerDeclaredBlockerWithNoItemEndsUnroutableWithItsOwnCode(t *testing.T) {
	t.Parallel()
	h := exhaustionHarness(t, PhaseBuild, codeReviewSpec(), "scout")

	outcome, diag, statement := h.cr.provenBlocker(PhaseBuild, ReviewInput{}, ReviewResult{ProvenBlocker: "blocked: needs console — proof"})

	if outcome != Unroutable || diag.Code != DiagCodeBlockerUnroutable || statement.empty() {
		t.Fatalf("outcome %d code %q statement %+v; a reviewer-declared blocker with no committed item is unroutable and keeps that code", outcome, diag.Code, statement)
	}
}

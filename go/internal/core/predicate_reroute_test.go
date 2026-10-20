package core

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const cycle1761Failure = "./acs/cycle10 (-tags acs): unit tests FAIL\n" +
	"--- FAIL: TestC1761_009_OnlyTargetPackagesTouched (0.02s)\n" +
	"    predicates_test.go:88: touched outside the allow-list: docs/explain/builds/cycle-1761.md, .evolve/evals/cycle-1761.md\n" +
	"FAIL\n" +
	"FAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle10\t0.239s\n" +
	"FAIL\n"

func realFloorFailure(perTest, pkg string, padding int) string {
	out := strings.Repeat("=== RUN   TestC1761_00x\n", padding) +
		"--- FAIL: TestC1761_009_OnlyTargetPackagesTouched (" + perTest + ")\n" +
		"    predicates_test.go:88: touched outside the allow-list: docs/explain/builds/cycle-1761.md, .evolve/evals/cycle-1761.md\n" +
		"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle10\t" + pkg + "\nFAIL\n"
	return "./acs/cycle10 (-tags acs): unit tests FAIL\n" + floorFailureDiagnostic(out)
}

type phaseOrderRunner struct {
	inner PhaseRunner
	seq   *[]string
	mu    *sync.Mutex
}

func (s phaseOrderRunner) Name() string { return s.inner.Name() }

func (s phaseOrderRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	s.mu.Lock()
	*s.seq = append(*s.seq, s.inner.Name())
	s.mu.Unlock()
	return s.inner.Run(ctx, req)
}

type changingFloor struct {
	mu    sync.Mutex
	calls int
	fixed func(call int) []string
}

func (f *changingFloor) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) {
		return ReviewResult{Approve: true}
	}
	f.mu.Lock()
	f.calls++
	failures := f.fixed(f.calls)
	f.mu.Unlock()
	if len(failures) == 0 {
		return ReviewResult{Approve: true}
	}
	return ReviewResult{Approve: false, Retry: true, Reason: "build handoff floor: " + strings.Join(failures, "\n"), Failures: failures}
}

type predicateRun struct {
	result CycleResult
	err    error
	seq    []string
	tdd    *fakeRunner
}

func runPredicateCycle(t *testing.T, floor DeliverableReviewer, extra ...Option) predicateRun {
	t.Helper()
	root := writeClaimableInbox(t, 1)
	repo := gittest.Fixture(t)
	var run predicateRun
	var mu sync.Mutex
	runners := buildRunners(nil)
	run.tdd = runners[PhaseTDD].(*fakeRunner)
	dispatches := 0
	runners[PhaseBuild] = claimingBuild{defects: []string{"the predicate's allow-list excludes the explanation document"}, dispatches: &dispatches}
	for p, r := range runners {
		runners[p] = phaseOrderRunner{inner: r, seq: &run.seq, mu: &mu}
	}
	runners[PhaseTriage] = phaseOrderRunner{inner: triageDecisionRunner{verdict: VerdictPASS, decision: oneCardDecision}, seq: &run.seq, mu: &mu}
	run.result, run.err = NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 9}}, &fakeLedger{}, runners,
		append([]Option{WithWorktreeProvisioner(&fakeWorktree{path: repo.Dir}), WithReviewer(floor), WithContractVerifier(&fakeVerifier{destSub: "build-report.md"})}, extra...)...).RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "lane-goal", Env: laneEnv,
	})
	return run
}

func phaseRuns(seq []string, phase Phase) []int {
	var at []int
	for i, p := range seq {
		if p == string(phase) {
			at = append(at, i)
		}
	}
	return at
}

func TestBuildFloor_Cycle1761ARepeatedTDDPredicateFailureRoutesToTDDNotBuild(t *testing.T) {
	t.Parallel()
	run := runPredicateCycle(t, &changingFloor{fixed: func(int) []string { return []string{cycle1761Failure} }})

	builds, tdds := phaseRuns(run.seq, PhaseBuild), phaseRuns(run.seq, PhaseTDD)
	if len(tdds) < 2 || len(builds) < 2 || tdds[1] != builds[1]+1 {
		t.Fatalf("sequence = %v; want TDD dispatched right after the first Build correction that repeated the predicate failure", run.seq)
	}
	brief := run.tdd.requests[1].Context[CtxKeyPredicateRepair]
	if !strings.Contains(brief, "TestC1761_009_OnlyTargetPackagesTouched") {
		t.Errorf("TDD re-dispatch context = %q, want the repeated predicate failure as its brief", brief)
	}
	if run.tdd.requests[0].Context[CtxKeyPredicateRepair] != "" {
		t.Error("the first TDD dispatch carried a predicate repair brief")
	}
	if len(tdds) != 2 {
		t.Errorf("TDD ran %d times; the reroute happens once per cycle, then the Build rounds end the cycle as before", len(tdds))
	}
}

func TestBuildFloor_APredicateFailureThatBuildMovesStillCorrectsAtBuild(t *testing.T) {
	t.Parallel()
	floor := &changingFloor{fixed: func(call int) []string {
		if call >= 3 {
			return nil
		}
		return []string{strings.Replace(cycle1761Failure, "docs/explain/builds/cycle-1761.md", "file-"+string(rune('a'+call))+".md", 1)}
	}}

	run := runPredicateCycle(t, floor)

	if tdds := phaseRuns(run.seq, PhaseTDD); len(tdds) != 1 {
		t.Errorf("sequence = %v; a predicate failure that changes between Build rounds stays at Build", run.seq)
	}
	if builds := phaseRuns(run.seq, PhaseBuild); len(builds) != 3 {
		t.Errorf("Build dispatched %d times, want 3: the first and two corrections", len(builds))
	}
}

func TestSamePredicateFailures_RealGoTestOutputOfTwoRunsIsTheSameFailure(t *testing.T) {
	t.Parallel()
	first := realFloorFailure("0.02s", "0.239s", 20)
	second := realFloorFailure("1.5s", "10.054s", 21)
	moved := strings.Replace(second, "cycle-1761.md, ", "", 1)

	if first == second || !strings.Contains(first, "…") {
		t.Fatalf("the fixture must be two truncated outputs that differ in their timings:\n%s\n%s", first, second)
	}
	if !samePredicateFailures([]string{first}, []string{second}) {
		t.Errorf("two runs of one failing predicate differ only in timings and in the cut of the tail, want the same failure:\n%s\n---\n%s", first, second)
	}
	if samePredicateFailures([]string{first}, []string{moved}) {
		t.Error("a changed assertion line compared as the same failure")
	}
}

func TestPredicateDefectRepeats_OnlyAnIdenticalPredicateFailureOfTheMainBuildDispatchOncePerCycle(t *testing.T) {
	t.Parallel()
	same := []string{cycle1761Failure}
	cases := []struct {
		name        string
		next        Phase
		remediation bool
		reroutes    int
		prev, cur   []string
		want        bool
	}{
		{"an identical failure after a Build correction", PhaseBuild, false, 0, same, same, true},
		{"the first rejection has nothing to compare", PhaseBuild, false, 0, nil, same, false},
		{"a changed failure", PhaseBuild, false, 0, same, []string{cycle1761Failure + " "}, false},
		{"only the test duration changed", PhaseBuild, false, 0, same, []string{strings.Replace(cycle1761Failure, "(0.02s)", "(1.5s)", 1)}, true},
		{"no predicate failure", PhaseBuild, false, 0, nil, nil, false},
		{"a remediation builder fix", PhaseBuild, true, 0, same, same, false},
		{"the reroute of this cycle is spent", PhaseBuild, false, maxPredicateReroutes, same, same, false},
		{"another phase", PhaseAudit, false, 0, same, same, false},
	}
	for _, c := range cases {
		cr := &cycleRun{cs: CycleState{PredicateReroutes: c.reroutes}}
		if got := cr.predicateDefectRepeats(c.next, &dispatchResult{remediation: c.remediation}, c.prev, c.cur); got != c.want {
			t.Errorf("%s: predicateDefectRepeats = %t, want %t", c.name, got, c.want)
		}
	}
}

func TestSeedAuditRepairContext_OnlyTDDGetsThePredicateRepairBrief(t *testing.T) {
	t.Parallel()
	cs := CycleState{PredicateRepairBrief: cycle1761Failure}
	if got := seedAuditRepairContext(nil, PhaseTDD, cs)[CtxKeyPredicateRepair]; got != cycle1761Failure {
		t.Errorf("TDD brief = %q", got)
	}
	if got := seedAuditRepairContext(nil, PhaseBuild, cs)[CtxKeyPredicateRepair]; got != "" {
		t.Errorf("Build got the TDD brief: %q", got)
	}
}

func TestConsumeDispatchFlags_EachDispatchSpendsItsOneShotFlag(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		next Phase
		want CycleState
	}{
		{PhaseAudit, CycleState{PredicateRepairBrief: "brief"}},
		{PhaseTDD, CycleState{AuditRepairActive: true}},
		{PhaseBuild, CycleState{AuditRepairActive: true, PredicateRepairBrief: "brief"}},
	} {
		cs := CycleState{AuditRepairActive: true, PredicateRepairBrief: "brief"}
		consumeDispatchFlags(&cs, c.next)
		if cs.AuditRepairActive != c.want.AuditRepairActive || cs.PredicateRepairBrief != c.want.PredicateRepairBrief {
			t.Errorf("%s: state = %+v, want %+v", c.next, cs, c.want)
		}
	}
}

func TestMaybeRemediate_ARepeatedPredicateFailureOfABuilderFixNeverReroutesToTDD(t *testing.T) {
	t.Parallel()
	reviewer := &remediateReviewerE4{decide: map[string]ReviewResult{"build": {Approve: false, Retry: true, Reason: "build handoff floor: " + cycle1761Failure, Failures: []string{cycle1761Failure}}}}
	rig := remediateRigE4(t, 1, reviewer, []string{VerdictPASS})
	rig.cr.o.stderr = &injectedStderr{}

	if _, err := rig.cr.maybeRemediate(PhaseTDD, rig.dr); err != nil {
		t.Fatalf("maybeRemediate: %v", err)
	}

	if rig.build.calls < 2 || rig.cr.scheduledNext == PhaseTDD || rig.cr.cs.PredicateReroutes != 0 {
		t.Errorf("builder fixes = %d, scheduled = %q, reroutes = %d; want the remediation fix corrected at Build and never rerouted", rig.build.calls, rig.cr.scheduledNext, rig.cr.cs.PredicateReroutes)
	}
}

func TestBuildFloor_TheRejectedBuildOfAReroutePassesThePostReviewGuards(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	leak := false
	dirty := func(context.Context, string) ([]string, error) {
		mu.Lock()
		defer mu.Unlock()
		if leak {
			return []string{"leaked-by-build.txt"}, nil
		}
		return nil, nil
	}
	floor := &changingFloor{fixed: func(call int) []string {
		if call == 2 {
			mu.Lock()
			leak = true
			mu.Unlock()
		}
		return []string{cycle1761Failure}
	}}

	run := runPredicateCycle(t, floor, WithGitDirtyPaths(dirty))

	if tdds := phaseRuns(run.seq, PhaseTDD); len(tdds) != 1 || run.err == nil || !strings.Contains(run.err.Error(), "tree") {
		t.Errorf("sequence = %v, err = %v; want the main-tree guard to stop the rerouted Build that leaked, before TDD runs again", run.seq, run.err)
	}
}

func TestAcsPredicateFailures_OnlyThePredicatesOfThisCycle(t *testing.T) {
	t.Parallel()
	other := strings.Replace(cycle1761Failure, "./acs/cycle10 ", "./acs/cycle9 ", 1)
	got := acsPredicateFailures([]string{cycle1761Failure, other, "gofmt: 1 file(s) unformatted"}, 10)
	if len(got) != 1 || got[0] != cycle1761Failure {
		t.Errorf("acsPredicateFailures = %q, want only cycle 10's predicate failure", got)
	}
}

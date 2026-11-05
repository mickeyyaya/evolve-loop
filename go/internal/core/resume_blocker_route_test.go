package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type resumedBuildRun struct {
	result     CycleResult
	err        error
	dispatches int
	workspace  string
	repoDir    string
	stderr     *strings.Builder
	signals    []signalcenter.Event
	directives []string
}

type resumedSetup struct {
	failures      []string
	defects       []string
	decision      string
	env           map[string]string
	retry         *policy.RetryConfig
	runner        func(*resumedBuildRun) PhaseRunner
	beforeResume  func(repoDir string)
	from          Phase
	base          func(t *testing.T, workspace, repoDir string)
	enforceLadder bool
}

func sealedBase(t *testing.T, workspace, repoDir string) {
	t.Helper()
	if err := writePredicateSnapshot(workspace, cyclePredicateSources(repoDir, 9)); err != nil {
		t.Fatal(err)
	}
}

func noBase(*testing.T, string, string) {}

func corruptBase(t *testing.T, workspace, _ string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workspace, predicateSnapshotFile), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
}

type directiveSpy struct {
	inner PhaseRunner
	run   *resumedBuildRun
}

func (d directiveSpy) Name() string { return d.inner.Name() }

func (d directiveSpy) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	d.run.directives = append(d.run.directives, req.CorrectionDirective)
	return d.inner.Run(ctx, req)
}

type lateClaimBuild struct {
	claimFrom int
	defects   []string
	run       *resumedBuildRun
}

func (lateClaimBuild) Name() string { return string(PhaseBuild) }

func (b lateClaimBuild) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	var defects []string
	if b.run.dispatches+1 >= b.claimFrom {
		defects = b.defects
	}
	return claimingBuild{defects: defects, dispatches: &b.run.dispatches}.Run(ctx, req)
}

type predicateEditingBuild struct {
	file    string
	defects []string
	run     *resumedBuildRun
}

func (predicateEditingBuild) Name() string { return string(PhaseBuild) }

func (b predicateEditingBuild) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	if err := os.WriteFile(b.file, []byte("package cycle9\n\n// the guard was weakened\n"), 0o644); err != nil {
		return PhaseResponse{}, err
	}
	return claimingBuild{defects: b.defects, dispatches: &b.run.dispatches}.Run(ctx, req)
}

func runResumedWith(t *testing.T, in resumedSetup) *resumedBuildRun {
	t.Helper()
	root := writeClaimableInbox(t, 1)
	repo := gittest.Fixture(t)
	for _, rel := range []string{selfcheckPath, breakerPath} {
		writeBlockerFile(t, repo.Dir, rel, "package core\n")
	}
	writeBlockerFile(t, repo.Dir, "go/acs/cycle9/predicates_test.go", strings.ReplaceAll(cycle1950Predicates, "cycle1950", "cycle9"))
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	ws := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if in.decision != "" {
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(in.decision), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := in.base
	if base == nil {
		base = sealedBase
	}
	base(t, ws, repo.Dir)
	if in.beforeResume != nil {
		in.beforeResume(repo.Dir)
	}
	from := in.from
	if from == "" {
		from = PhaseBuild
	}
	run := &resumedBuildRun{workspace: ws, repoDir: repo.Dir, stderr: &strings.Builder{}}
	wt := &fakeWorktree{path: repo.Dir}
	st := &fakeStorage{state: State{LastCycleNumber: 9}, cycleState: CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: wt.path}}
	runners := buildRunners(nil)
	var build PhaseRunner = claimingBuild{defects: in.defects, dispatches: &run.dispatches}
	if in.runner != nil {
		build = in.runner(run)
	}
	runners[PhaseBuild] = directiveSpy{inner: build, run: run}
	center := signalcenter.New()
	var mu sync.Mutex
	center.Subscribe(func(e signalcenter.Event) {
		mu.Lock()
		defer mu.Unlock()
		run.signals = append(run.signals, e)
	})
	if in.enforceLadder {
		writeCLIProfile(t, root, "builder", "agy-tmux", nil)
	}
	opts := []Option{WithWorktreeProvisioner(wt), WithReviewer(floorRejecting{failures: in.failures}), WithProtectedSurface(protectsSelfcheck), WithStderr(run.stderr), WithSignalCenter(center)}
	if in.retry != nil {
		opts = append(opts, WithRetryConfig(*in.retry))
	}
	if in.enforceLadder {
		opts = append(opts, WithWorkflowConfig(ladderWorkflow(policy.RetryLadderEnforce)))
	}
	o := NewOrchestrator(st, &fakeLedger{}, runners, opts...)
	run.result, run.err = o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "lane-goal", Env: in.env}, &ResumePoint{Phase: string(from), CycleID: 9})
	center.Flush()
	return run
}

func runResumedBuild(t *testing.T, failures, defects []string, decision string, env map[string]string) *resumedBuildRun {
	t.Helper()
	return runResumedWith(t, resumedSetup{failures: failures, defects: defects, decision: decision, env: env})
}

func (r *resumedBuildRun) signal(code signalcenter.Code) (signalcenter.Event, bool) {
	for _, e := range r.signals {
		if e.Code == code {
			return e, true
		}
	}
	return signalcenter.Event{}, false
}

func (r *resumedBuildRun) stderrHas(t *testing.T, line string) {
	t.Helper()
	if !strings.Contains(r.stderr.String(), line) {
		t.Errorf("stderr = %q\nwant the line %q", r.stderr.String(), line)
	}
}

func TestResumedBuild_AProtectedPathBlockerRoutesTheItemToTheConsoleWithNoFurtherCorrection(t *testing.T) {
	t.Parallel()
	run := runResumedBuild(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, oneCardDecision, laneEnv)

	if run.err != nil {
		t.Fatalf("RunCycleFromPhase: %v; a proven blocker is a planned ending on resume too", run.err)
	}
	if run.result.TerminationReason != CycleTerminationBlockedNeedsConsole || !IsProvenBlockerResult(run.result) {
		t.Fatalf("result = %+v; want the blocked-needs-console ending", run.result)
	}
	if run.dispatches != 1 {
		t.Errorf("build dispatched %d times; a proven blocker spends no correction round", run.dispatches)
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
	want := "blocked: needs console — phase build stopped on a proven blocker the lane cannot remove: protected-path: the fix needs " + selfcheckPath + ", a protected control-plane path (the lane said: " + cycle1946Claim + ")"
	if msg := last.Diagnostics[0].Message; msg != want {
		t.Errorf("route message = %q\nwant %q", msg, want)
	}
}

func TestResumedCorrector_CorrectCallsTheProvenBlockerRule(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(oneCardDecision), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{name: string(PhaseBuild)}
	rv := resumedCorrector{
		o: NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)), ctx: context.Background(),
		scope:  phaseLeakScope{cycleState: CycleState{CycleID: 9, WorkspacePath: ws}, phase: PhaseBuild},
		cycle:  9,
		runner: runner,
		count:  func(_ Phase, r PhaseRequest) PhaseRequest { return r },
	}
	_, err := rv.correct(PhaseRequest{}, PhaseResponse{Phase: "build", Verdict: VerdictFAIL}, ReviewResult{Retry: true, Reason: "floor", ProvenBlocker: "blocked: needs console — proof"})

	var blocked resumedBlocker
	if !errors.As(err, &blocked) || blocked.diag.Message != "blocked: needs console — proof" || blocked.diag.Subject != laneScopedItem {
		t.Fatalf("err = %v; a proven blocker on the resume path routes %s with the proof as written", err, laneScopedItem)
	}
	if len(runner.requests) != 0 {
		t.Errorf("a proven blocker spends no correction round, dispatched %d", len(runner.requests))
	}
}

func TestResumedBuild_AnUnprovenBlockerIsRefusedAndTheCorrectionsContinue(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	run := runResumedBuild(t, []string{cycle1950Failure}, []string{fake}, oneCardDecision, laneEnv)

	if run.result.TerminationReason == CycleTerminationBlockedNeedsConsole {
		t.Fatalf("result = %+v; a claim the floor cannot prove must not route", run.result)
	}
	if run.err == nil || run.dispatches < 2 {
		t.Errorf("err=%v dispatches=%d; a refused claim keeps the correction rounds", run.err, run.dispatches)
	}
	if !strings.Contains(run.stderr.String(), "the declared blocker is not proven") {
		t.Errorf("stderr = %q; the refusal names its cause", run.stderr.String())
	}
}

func TestResumedBuild_NoRouteWithoutOneCommittedItem(t *testing.T) {
	t.Parallel()
	twoCards := `{"top_n":[{"id":"claimable-lane-item"},{"id":"other-item"}]}`
	run := runResumedBuild(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, twoCards, nil)

	if run.result.TerminationReason == CycleTerminationBlockedNeedsConsole {
		t.Errorf("result = %+v; the console route names exactly one item", run.result)
	}
	if !strings.Contains(run.stderr.String(), "a console route needs exactly one") {
		t.Errorf("stderr = %q; the refusal says why", run.stderr.String())
	}
}

func TestResumedBuild_AClaimInTheLastCorrectionStillRoutes(t *testing.T) {
	t.Parallel()
	run := runResumedWith(t, resumedSetup{
		failures: []string{cycle1946PredicateFailure}, decision: oneCardDecision, env: laneEnv,
		retry: &policy.RetryConfig{PhaseMaxAttempts: 1, ContractCorrectionRetries: 2},
		runner: func(r *resumedBuildRun) PhaseRunner {
			return lateClaimBuild{claimFrom: 3, defects: []string{cycle1946Claim}, run: r}
		},
	})

	if run.err != nil || run.result.TerminationReason != CycleTerminationBlockedNeedsConsole {
		t.Fatalf("err=%v result=%+v; a claim in the last correction is proven before the budget ends the cycle", run.err, run.result)
	}
	if run.dispatches != 3 {
		t.Errorf("build dispatched %d times, want 3", run.dispatches)
	}
}

func TestResumedBuild_AClaimInTheFirstReportRoutesWithZeroCorrections(t *testing.T) {
	t.Parallel()
	run := runResumedWith(t, resumedSetup{
		failures: []string{cycle1946PredicateFailure}, defects: []string{cycle1946Claim}, decision: oneCardDecision, env: laneEnv,
		retry: &policy.RetryConfig{PhaseMaxAttempts: 1, ContractCorrectionRetries: 0},
	})

	if run.err != nil || run.result.TerminationReason != CycleTerminationBlockedNeedsConsole {
		t.Fatalf("err=%v result=%+v; the proof runs before the budget, so zero corrections still route", run.err, run.result)
	}
	if run.dispatches != 1 {
		t.Errorf("build dispatched %d times, want 1", run.dispatches)
	}
}

func TestResumedBuild_PredicatesEditedBeforeTheResumeAreRefused(t *testing.T) {
	t.Parallel()
	run := runResumedWith(t, resumedSetup{
		failures: []string{cycle1946PredicateFailure}, defects: []string{cycle1946Claim}, decision: oneCardDecision, env: laneEnv,
		beforeResume: func(dir string) {
			writeBlockerFile(t, dir, "go/acs/cycle9/predicates_test.go", "package cycle9\n\n// edited by an earlier dispatch\n")
		},
	})

	if run.result.TerminationReason == CycleTerminationBlockedNeedsConsole {
		t.Fatalf("result = %+v; an edit made before the resume must not prove a blocker", run.result)
	}
	run.stderrHas(t, "the lane changed the predicates of the cycle after the phase request")
}

func TestResumedBuild_PredicatesEditedDuringTheResumedBuildAreRefused(t *testing.T) {
	t.Parallel()
	run := runResumedWith(t, resumedSetup{
		failures: []string{cycle1946PredicateFailure}, decision: oneCardDecision, env: laneEnv,
		runner: func(r *resumedBuildRun) PhaseRunner {
			return predicateEditingBuild{file: filepath.Join(r.repoDir, "go", "acs", "cycle9", "predicates_test.go"), defects: []string{cycle1946Claim}, run: r}
		},
	})

	if run.result.TerminationReason == CycleTerminationBlockedNeedsConsole {
		t.Fatalf("result = %+v; a predicate edit made by the build must not prove a blocker", run.result)
	}
	run.stderrHas(t, "the lane changed the predicates of the cycle after the phase request")
}

func TestResumedBuild_TheRoutedSignalCarriesTheExactProof(t *testing.T) {
	t.Parallel()
	run := runResumedBuild(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, oneCardDecision, laneEnv)

	ev, ok := run.signal(CodeBlockerRouted)
	if !ok {
		t.Fatalf("signals = %v; want %s", run.signals, CodeBlockerRouted)
	}
	want := "blocked: needs console — phase build stopped on a proven blocker the lane cannot remove: protected-path: the fix needs " + selfcheckPath + ", a protected control-plane path (the lane said: " + cycle1946Claim + ")"
	if ev.Reason != want {
		t.Errorf("reason = %q\nwant %q", ev.Reason, want)
	}
}

func TestResumedBuild_TheRefusedSignalAndTheNextDirectiveCarryTheExactRefusal(t *testing.T) {
	t.Parallel()
	fake := "BLOCKER protected-path " + breakerPath + ":76 I would rather not test this"
	run := runResumedBuild(t, []string{cycle1950Failure}, []string{fake}, oneCardDecision, laneEnv)

	ev, ok := run.signal(CodeBlockerRefused)
	if !ok {
		t.Fatalf("signals = %v; want %s", run.signals, CodeBlockerRefused)
	}
	want := "the declared blocker is not proven: \"" + fake + "\": " + breakerPath + " is not on the protected-surface manifest, so the lane can edit it. Next: correct the BLOCKER line of your report so each claim is true, or fix the failures without declaring a blocker."
	if ev.Reason != want {
		t.Errorf("reason = %q\nwant %q", ev.Reason, want)
	}
	run.stderrHas(t, "[orchestrator] phase build: "+want+"\n")
	if len(run.directives) < 2 || !strings.Contains(run.directives[1], "The host refused your BLOCKER claim: "+want) {
		t.Errorf("directives = %q; the correction tells the lane why the host refused the claim", run.directives)
	}
}

func TestResumedBuild_NoRouteTextsNameTheCauseTheEvidenceAndTheNextStep(t *testing.T) {
	t.Parallel()
	step := "at the boundary, the console routes each item that needs this path with evolve inbox route-console <id> <reason> <cycle>. A quarantined item needs evolve inbox quarantine release <id> first."
	cases := []struct {
		name, decision string
		want           func(ws string) string
	}{
		{"two cards", `{"top_n":[{"id":"claimable-lane-item"},{"id":"other-item"}]}`, func(string) string {
			return "the blocker is proven, and the triage decision commits items claimable-lane-item, other-item, and a console route needs exactly one, so this cycle ends with no route. Next: " + step
		}},
		{"no decision", "", func(ws string) string {
			return "the blocker is proven, and neither " + filepath.Join(ws, "lane-scope.json") + " nor " + filepath.Join(ws, "triage-decision.json") + " gives a commitment, so this cycle ends with no route. Next: the console reads both paths."
		}},
		{"empty decision", `{"top_n":[]}`, func(ws string) string {
			return "the blocker is proven, and the triage decision commits no item, so this cycle ends with no route. Next: the console reads " + filepath.Join(ws, "triage-decision.json") + "."
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			run := runResumedBuild(t, []string{cycle1946PredicateFailure}, []string{cycle1946Claim}, tc.decision, nil)
			ev, ok := run.signal(CodeBlockerRefused)
			want := tc.want(run.workspace)
			if !ok || ev.Reason != want {
				t.Errorf("signal = %+v ok=%v\nwant reason %q", ev, ok, want)
			}
			run.stderrHas(t, "[orchestrator] phase build: "+want+"\n")
		})
	}
}

type blockerEndCase struct {
	name     string
	decision string
	env      map[string]string
	base     func(*testing.T, string, string)
	outcome  BlockerOutcome
	code     string
	fresh    bool
}

var blockerEndCases = []blockerEndCase{
	{"unroutable: two items committed", `{"top_n":[{"id":"claimable-lane-item"},{"id":"other-item"}]}`, nil, nil, Unroutable, cyclestate.DiagCodeBlockerUnroutable, true},
	{"unroutable: no item committed", `{"top_n":[]}`, nil, nil, Unroutable, cyclestate.DiagCodeBlockerUnroutable, false},
	{"unroutable: no commitment known", "", nil, nil, Unroutable, cyclestate.DiagCodeBlockerUnroutable, true},
	{"incomplete: the base is missing", oneCardDecision, laneEnv, noBase, Incomplete, cyclestate.DiagCodeBlockerIncomplete, false},
	{"incomplete: the base is corrupt", oneCardDecision, laneEnv, corruptBase, Incomplete, cyclestate.DiagCodeBlockerIncomplete, false},
}

func TestResumedBuild_AnOutcomeThatNoCorrectionCanClearEndsAfterTheFirstDispatchWithNoDirective(t *testing.T) {
	t.Parallel()
	for _, tc := range blockerEndCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			run := runResumedWith(t, resumedSetup{failures: []string{cycle1946PredicateFailure}, defects: []string{cycle1946Claim}, decision: tc.decision, env: tc.env, base: tc.base})

			var ended blockerEnd
			if !errors.As(run.err, &ended) || ended.outcome != tc.outcome {
				t.Fatalf("err = %v; want the typed end of outcome %d", run.err, tc.outcome)
			}
			if run.dispatches != 1 {
				t.Errorf("build dispatched %d times; the %s end spends no correction round", run.dispatches, tc.name)
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
			if codes := cyclestate.ErrorCodes(last.Diagnostics); last.Verdict != string(VerdictFAIL) || len(codes) == 0 || codes[0] != tc.code {
				t.Errorf("last timing entry = %+v; want the FAIL outcome to lead with %s", last, tc.code)
			}
		})
	}
}

func TestResumedBuild_AResumeFromScoutSealsTheBaseAtItsOwnBuildEntryAndRoutes(t *testing.T) {
	t.Parallel()
	run := runResumedWith(t, resumedSetup{
		from: PhaseScout, base: noBase,
		failures: []string{cycle1946PredicateFailure}, defects: []string{cycle1946Claim}, decision: oneCardDecision, env: laneEnv,
	})

	if run.err != nil || !IsProvenBlockerResult(run.result) {
		t.Fatalf("err=%v result=%+v; committed and unedited predicates with a proven claim route", run.err, run.result)
	}
	if run.dispatches != 1 {
		t.Errorf("build dispatched %d times; a proven blocker spends no correction round", run.dispatches)
	}
	base, fault := readPredicateSnapshot(run.workspace)
	if fault != "" || len(base) == 0 {
		t.Errorf("the resume loop stored no base: %v, %q", base, fault)
	}
}

func TestResumedBuild_ASealFailureAtTheBuildEntryAbortsBeforeAnyDispatch(t *testing.T) {
	t.Parallel()
	blockedSeal := func(t *testing.T, workspace, _ string) {
		t.Helper()
		dir := filepath.Join(workspace, predicateSnapshotFile)
		if err := os.MkdirAll(filepath.Join(dir, "keep"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	run := runResumedWith(t, resumedSetup{
		from: PhaseScout, base: blockedSeal,
		failures: []string{cycle1946PredicateFailure}, defects: []string{cycle1946Claim}, decision: oneCardDecision, env: laneEnv,
	})

	if run.err == nil || !strings.Contains(run.err.Error(), "resume seal predicate base") {
		t.Fatalf("err = %v; a seal failure at the Build entry must abort the resume", run.err)
	}
	if run.dispatches != 0 {
		t.Errorf("build dispatched %d times; a failed seal aborts before any dispatch", run.dispatches)
	}
}

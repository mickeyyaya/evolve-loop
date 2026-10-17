package core

import (
	"context"
	"errors"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type scriptedRunner struct {
	name       Phase
	verdicts   []string
	calls      int
	directives []string
}

func (r *scriptedRunner) Name() string { return string(r.name) }
func (r *scriptedRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	i := r.calls
	if i >= len(r.verdicts) {
		i = len(r.verdicts) - 1
	}
	r.calls++
	r.directives = append(r.directives, req.CorrectionDirective)
	return PhaseResponse{Phase: string(r.name), Verdict: r.verdicts[i], ArtifactsDir: req.Workspace}, nil
}

func remediationHarness(t *testing.T, wf policy.WorkflowConfig, gate Phase, gateVerdicts []string) (*scriptedRunner, *scriptedRunner, CycleResult, error) {
	t.Helper()
	runners := buildRunners(nil)
	gr := &scriptedRunner{name: gate, verdicts: gateVerdicts}
	build := &scriptedRunner{name: PhaseBuild, verdicts: []string{VerdictPASS}}
	runners[gate] = gr
	runners[PhaseBuild] = build
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorkflowConfig(wf))
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	return gr, build, res, err
}

func remediationDispatches(build *scriptedRunner) []string {
	var out []string
	for _, d := range build.directives {
		if strings.Contains(d, "REMEDIATION") {
			out = append(out, d)
		}
	}
	return out
}

func TestRemediation_GateFailThenPassContinuesSpine(t *testing.T) {
	t.Parallel()

	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	gate, build, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictPASS})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	rems := remediationDispatches(build)
	if len(rems) != 1 {
		t.Fatalf("want exactly 1 remediation builder dispatch, got %d (directives: %q)", len(rems), build.directives)
	}
	wantReport := phasecontract.ArtifactFilename("tdd")
	if wantReport == "tdd-report.md" {
		t.Fatalf("registry regression: tdd's artifact is %q — this test's whole point is that the gate's "+
			"real deliverable differs from the <phase>-report.md convention", wantReport)
	}
	if !strings.Contains(rems[0], "tdd") || !strings.Contains(rems[0], wantReport) {
		t.Errorf("remediation directive must name the gate and its REGISTRY artifact %q; got %q", wantReport, rems[0])
	}
	if strings.Contains(rems[0], "tdd-report.md") {
		t.Errorf("remediation directive still names the conventional tdd-report.md, which the gate never writes; got %q", rems[0])
	}
	if gate.calls != 2 {
		t.Fatalf("gate must re-run after the fix: calls=%d, want 2 (remediated PASS ends the retry pressure)", gate.calls)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Fatalf("remediated cycle must continue to a PASS verdict; got %q", res.FinalVerdict)
	}
	if len(res.Remediations) != 1 || !strings.Contains(res.Remediations[0], "tdd") || !strings.Contains(res.Remediations[0], VerdictPASS) {
		t.Errorf("provenance must record the remediated gate + outcome; got %v", res.Remediations)
	}
}

type buildReviewCounter struct{ builds int }

func (r *buildReviewCounter) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase == string(PhaseBuild) {
		r.builds++
	}
	return ReviewResult{Approve: true}
}

func TestRemediation_BuilderFixPassesThroughBuildReviewChain(t *testing.T) {
	t.Parallel()

	runners := buildRunners(nil)
	gate := &scriptedRunner{name: PhaseTDD, verdicts: []string{VerdictFAIL, VerdictPASS}}
	build := &scriptedRunner{name: PhaseBuild, verdicts: []string{VerdictPASS}}
	runners[PhaseTDD], runners[PhaseBuild] = gate, build
	reviews := &buildReviewCounter{}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorkflowConfig(policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}),
		WithReviewer(reviews))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if reviews.builds != 2 {
		t.Fatalf("Build review calls=%d, want 2 (normal Build + remediation Builder fix)", reviews.builds)
	}
}

func TestRemediation_RoundCapIsHard(t *testing.T) {
	t.Parallel()

	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	gate, build, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := len(remediationDispatches(build)); got != 1 {
		t.Fatalf("cap=1 must mean exactly 1 remediation dispatch, got %d", got)
	}
	if gate.calls < 2 {
		t.Fatalf("gate calls=%d, want >=2 (original + the one remediation re-run)", gate.calls)
	}
	if len(res.Remediations) != 1 || !strings.Contains(res.Remediations[0], VerdictFAIL) {
		t.Errorf("provenance must record the failed remediation; got %v", res.Remediations)
	}
}

func TestRemediation_ZeroConfigIsByteIdenticalLegacy(t *testing.T) {
	t.Parallel()

	_, build, res, err := remediationHarness(t, policy.WorkflowConfig{}, PhaseTDD, []string{VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := len(remediationDispatches(build)); got != 0 {
		t.Fatalf("zero-value config must never remediate; got %d dispatches", got)
	}
	if len(res.Remediations) != 0 {
		t.Errorf("no remediation provenance expected; got %v", res.Remediations)
	}
}

func TestRemediation_UnlistedPhaseUntouched(t *testing.T) {
	t.Parallel()

	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"coverage-gate"}}
	_, build, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := len(remediationDispatches(build)); got != 0 {
		t.Fatalf("audit is not in the remediable list; got %d dispatches", got)
	}
	if len(res.Remediations) != 0 {
		t.Errorf("no remediation provenance for an unlisted phase; got %v", res.Remediations)
	}
}

func TestRemediation_JudgmentPhasesDeniedRegardlessOfConfig(t *testing.T) {
	t.Parallel()

	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"audit"}}
	_, build, res, err := remediationHarness(t, wf, PhaseAudit, []string{VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := len(remediationDispatches(build)); got != 0 {
		t.Fatalf("judgment phase must never remediate; got %d dispatches", got)
	}
	if len(res.Remediations) != 0 {
		t.Errorf("no provenance for a denied phase; got %v", res.Remediations)
	}
}

func TestRemediation_RecordsFixDispatchInPhaseRecord(t *testing.T) {
	t.Parallel()

	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	_, _, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictPASS})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	found := false
	for _, p := range res.PhasesRun {
		if p == Phase("build-remediation") {
			found = true
		}
	}
	if !found {
		t.Fatalf("PhasesRun must record the remediation fix dispatch; got %v", res.PhasesRun)
	}
}

type diagRunner struct{ name Phase }

func (r *diagRunner) Name() string { return string(r.name) }
func (r *diagRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: string(r.name), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace,
		Diagnostics: []Diagnostic{{Severity: "error", Message: "apicover -enforce flagged 1 line(s) — unnamed export"}}}, nil
}

func TestFailReasonsSurfaceInResult(t *testing.T) {
	t.Parallel()

	runners := buildRunners(nil)
	runners[PhaseAudit] = &diagRunner{name: PhaseAudit}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)
	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	found := false
	for _, r := range res.FailReasons {
		if strings.Contains(r, "apicover") {
			found = true
		}
	}
	if !found {
		t.Fatalf("FailReasons must surface the override explanation; got %v", res.FailReasons)
	}
}

func TestDispatch_ReadOnlyPhasesAreFencedAndSourceWritersAreNot(t *testing.T) {
	t.Parallel()

	runners := buildRunners(nil)
	gate := &scriptedRunner{name: PhaseScout, verdicts: []string{VerdictFAIL, VerdictPASS}}
	runners[PhaseScout] = gate
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorkflowConfig(policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"scout"}}))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	build := runners[PhaseBuild].(*fakeRunner)
	if len(build.requests) < 2 {
		t.Fatalf("build dispatched %d time(s); want the remediation fix + the normal build", len(build.requests))
	}
	for i, r := range build.requests {
		if r.WorktreeReadOnly {
			t.Errorf("build request %d is fenced — a source writer (and a remediation fix inheriting a read-only gate's request) must keep its writes", i)
		}
	}
	for _, p := range []Phase{PhaseAudit, PhaseTriage} {
		fr := runners[p].(*fakeRunner)
		if len(fr.requests) == 0 || !fr.requests[0].WorktreeReadOnly {
			t.Errorf("%s is not a declared source writer and must be dispatched fenced", p)
		}
	}
	if tdd := runners[PhaseTDD].(*fakeRunner); len(tdd.requests) > 0 && tdd.requests[0].WorktreeReadOnly {
		t.Error("tdd writes source and must not be fenced")
	}
}

func TestDispatch_ReadOnlyFlagOnResumeAndEvaluateBatchSurfaces(t *testing.T) {
	t.Parallel()

	runners := buildRunners(map[Phase]string{PhaseAudit: VerdictPASS})
	root := t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: CycleState{CycleID: 41, Phase: string(PhaseAudit), WorkspacePath: RunWorkspacePath(root, 41)}}
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 41}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	audit := runners[PhaseAudit].(*fakeRunner)
	if len(audit.requests) == 0 || !audit.requests[0].WorktreeReadOnly {
		t.Errorf("a resumed audit must be dispatched fenced (resume.go request builder)")
	}

	batchRunners := buildRunners(nil)
	tester := &fakeRunner{name: "tester", verdict: VerdictPASS}
	evaluator := &fakeRunner{name: "evaluator", verdict: VerdictPASS}
	batchRunners[Phase("tester")], batchRunners[Phase("evaluator")] = tester, evaluator
	cr := newBatchCycleRun(t, batchRunners, 2)
	if act, err := cr.dispatchEvaluateBatch([]Phase{"tester", "evaluator"}); err != nil || act != loopNext {
		t.Fatalf("batch: act=%v err=%v", act, err)
	}
	for _, fr := range []*fakeRunner{tester, evaluator} {
		if len(fr.requests) == 0 || !fr.requests[0].WorktreeReadOnly {
			t.Errorf("evaluate-batch phase %s must be dispatched fenced (evaluate_batch.go request builder)", fr.name)
		}
	}
}

type cancelProbeRunner struct {
	obs          *recordingObserver
	cancelsAtRun int32
}

func (r *cancelProbeRunner) Name() string { return "probe" }
func (r *cancelProbeRunner) Run(_ context.Context, _ PhaseRequest) (PhaseResponse, error) {
	r.cancelsAtRun = r.obs.cancelCalls.Load()
	return PhaseResponse{Verdict: VerdictPASS}, nil
}

func TestObservedRun_TheObserverWatchesTheWholeRun(t *testing.T) {
	t.Parallel()

	obs := &recordingObserver{}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithObserver(obs))
	cr := &cycleRun{o: o, ctx: context.Background()}
	runner := &cancelProbeRunner{obs: obs}

	if _, err := cr.observedRun(PhaseBuild, runner, PhaseRequest{}); err != nil {
		t.Fatal(err)
	}

	if runner.cancelsAtRun != 0 || obs.cancelCalls.Load() != 1 || len(obs.starts) != 1 || obs.starts[0] != string(PhaseBuild) {
		t.Fatalf("observer: starts=%v cancels during run=%d after=%d, want one %s watch spanning the run, cancelled once after it", obs.starts, runner.cancelsAtRun, obs.cancelCalls.Load(), PhaseBuild)
	}
}

func TestRemediation_FixDirectiveAsksForTheRoundsNumberedCorrectionRecord(t *testing.T) {
	t.Parallel()

	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	gate, build, _, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictPASS})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	rems := remediationDispatches(build)
	if len(rems) != 1 {
		t.Fatalf("want 1 remediation dispatch, got %d", len(rems))
	}
	if want := correctionRecord(1); !strings.HasSuffix(rems[0], want) {
		t.Errorf("remediation directive must end with the numbered correction record %q\n  got: %s", want, rems[0])
	}
	heading := "## Correction 1"
	if !correctionHeadingRE.MatchString(heading) || !strings.Contains(rems[0], "`"+heading+"`") {
		t.Errorf("the requested heading must be one the build floor parses as a correction section: %q", rems[0])
	}
	reverify := gate.directives[len(gate.directives)-1]
	if want := "RE-VERIFY after remediation round 1: run your full check fresh against the current worktree."; reverify != want {
		t.Errorf("the read-only gate re-verify directive must stay unchanged, got %q", reverify)
	}
}

func TestRemediation_RequestedRecordRoundTripsThroughThePlaceholderCheck(t *testing.T) {
	t.Parallel()

	body := "# Build Report\n\nresult: ok\n\n## Correction 1\nRemoved the `CMD_EVOLVE_PLACEHOLDER` slot flagged at build-report.md:62.\n"
	if got := placeholderFailuresFor(t, body); len(got) != 0 {
		t.Fatalf("a record in the requested format must parse as a correction section; got %v", got)
	}
}

type remediationErrRunner struct {
	name  Phase
	calls int
	errAt int
	err   error
}

func (r *remediationErrRunner) Name() string { return string(r.name) }
func (r *remediationErrRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	if r.calls == r.errAt {
		return PhaseResponse{}, r.err
	}
	verdict := VerdictFAIL
	if r.calls > 1 {
		verdict = VerdictPASS
	}
	return PhaseResponse{Phase: r.name.String(), Verdict: verdict, ArtifactsDir: req.Workspace}, nil
}

func remediationWithErrRunner(t *testing.T, gateErrAt, buildErrAt int, buildVerdicts []string) CycleResult {
	t.Helper()
	runners := buildRunners(nil)
	runners[PhaseTDD] = &remediationErrRunner{name: PhaseTDD, errAt: gateErrAt, err: errors.New("gate boom")}
	if buildErrAt > 0 {
		runners[PhaseBuild] = &remediationErrRunner{name: PhaseBuild, errAt: buildErrAt, err: errors.New("builder boom")}
	} else {
		runners[PhaseBuild] = &scriptedRunner{name: PhaseBuild, verdicts: buildVerdicts}
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorkflowConfig(policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}))
	res, _ := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	return res
}

func TestRemediation_BuilderDispatchErrorLeavesTheGateFailStanding(t *testing.T) {
	t.Parallel()

	res := remediationWithErrRunner(t, 0, 1, nil)
	if len(res.Remediations) == 0 || !strings.Contains(res.Remediations[0], "builder-failed") {
		t.Fatalf("a builder dispatch error must be recorded as builder-failed; got %v", res.Remediations)
	}
}

func TestRemediation_BuilderFailVerdictLeavesTheGateFailStanding(t *testing.T) {
	t.Parallel()

	res := remediationWithErrRunner(t, 0, 0, []string{VerdictFAIL})
	if len(res.Remediations) == 0 || !strings.Contains(res.Remediations[0], "builder-failed") {
		t.Fatalf("a builder FAIL verdict must be recorded as builder-failed; got %v", res.Remediations)
	}
}

func TestRemediation_GateReRunDispatchErrorLeavesTheOriginalFailStanding(t *testing.T) {
	t.Parallel()

	res := remediationWithErrRunner(t, 2, 0, []string{VerdictPASS})
	if len(res.Remediations) == 0 || !strings.Contains(res.Remediations[0], "re-run dispatch-failed") {
		t.Fatalf("a failed gate re-run must be recorded as dispatch-failed; got %v", res.Remediations)
	}
}

func TestSnapshotThenRun_ASnapshotFailureNeverDispatchesTheRunner(t *testing.T) {
	t.Parallel()

	reader := &scriptedDirtyReader{err: errors.New("status unreadable")}
	cr := snapshotCycleRun(t, checkoutRoot(t), reader)
	runner := &scriptedRunner{name: PhaseBuild, verdicts: []string{VerdictPASS}}

	_, _, _, err := cr.snapshotThenRun(PhaseBuild, runner, PhaseRequest{})

	if err == nil || runner.calls != 0 {
		t.Fatalf("a failed pre-phase snapshot must stop the dispatch: err=%v calls=%d", err, runner.calls)
	}
}

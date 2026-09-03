package core

// cyclerun_remediate_test.go — graduated remediation (operator directive
// 2026-07-21; inbox graduated-remediation-fix-forward): when a configured
// DETERMINISTIC gate phase FAILs, the orchestrator dispatches the builder ONCE
// with the gate's report as a correction directive, re-runs the SAME gate, and
// records the final verdict — instead of discarding a sound cycle over a
// mechanical, prescribed defect (the 983/992/1007/1019/1020 waste class:
// cycle-1019's audit-PASSed S5 implementation was thrown away over three
// missing test files the gate itself had prescribed; 1020 then re-implemented
// it from scratch and failed the same gate the same way).
//
// Integrity properties pinned here: nothing downstream is bypassed (the SAME
// gate must pass and the spine continues normally); the round cap is hard; a
// zero-value workflow config means ZERO remediation (byte-identical legacy
// behavior — compiled defaults live at the composition root, not in core).

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// scriptedRunner returns queued verdicts call-by-call (last repeats) and
// records every CorrectionDirective it was dispatched with.
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

// remediationDispatches counts builder calls that carried a REMEDIATION
// directive (the spine's own build phase runs with an empty one).
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
	// tdd stands in for a deterministic gate: spine-reachable with the fake
	// runner map and NOT on the judgment deny-list (which the mechanism
	// enforces regardless of configuration — pinned separately below).
	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	gate, build, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictPASS})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	rems := remediationDispatches(build)
	if len(rems) != 1 {
		t.Fatalf("want exactly 1 remediation builder dispatch, got %d (directives: %q)", len(rems), build.directives)
	}
	// The report filename must come from the phasecontract registry, NOT the
	// "<phase>-report.md" convention: the tdd gate's deliverable is
	// test-report.md, so the convention pointed the remediation directive at a
	// tdd-report.md that never exists — the builder was told to read a missing
	// file. Asserting through ArtifactFilename keeps this honest under a rename.
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
	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}
	gate, build, res, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if got := len(remediationDispatches(build)); got != 1 {
		t.Fatalf("cap=1 must mean exactly 1 remediation dispatch, got %d", got)
	}
	// After the capped remediation fails, the cycle proceeds down the SAME
	// legacy path as an unremediated FAIL (retro / fluent-vs-strict semantics
	// own the final verdict — remediation never overrides them). The audit may
	// re-run additional times via the legacy retry loop; the cap governs
	// REMEDIATION dispatches only.
	if gate.calls < 2 {
		t.Fatalf("gate calls=%d, want >=2 (original + the one remediation re-run)", gate.calls)
	}
	if len(res.Remediations) != 1 || !strings.Contains(res.Remediations[0], VerdictFAIL) {
		t.Errorf("provenance must record the failed remediation; got %v", res.Remediations)
	}
}

func TestRemediation_ZeroConfigIsByteIdenticalLegacy(t *testing.T) {
	_, build, res, err := remediationHarness(t, policy.WorkflowConfig{}, PhaseTDD, []string{VerdictFAIL})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// Byte-identical legacy = the DELTAS are zero: no remediation dispatches,
	// no provenance. (The final verdict and audit retry count belong to the
	// legacy fluent/strict path and are pinned by the existing orchestrator
	// tests, not re-asserted here.)
	if got := len(remediationDispatches(build)); got != 0 {
		t.Fatalf("zero-value config must never remediate; got %d dispatches", got)
	}
	if len(res.Remediations) != 0 {
		t.Errorf("no remediation provenance expected; got %v", res.Remediations)
	}
}

func TestRemediation_UnlistedPhaseUntouched(t *testing.T) {
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

// TestRemediation_JudgmentPhasesDeniedRegardlessOfConfig pins the deny-list:
// configuring a judgment phase (audit) as remediable is REFUSED by the
// mechanism itself — a builder re-roll against an LLM-judgment verdict would
// be a gamed verdict, not a fix.
func TestRemediation_JudgmentPhasesDeniedRegardlessOfConfig(t *testing.T) {
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

// TestRemediation_RecordsFixDispatchInPhaseRecord pins the ADR-0044 C1
// chokepoint parity: the remediation fix dispatch appears in the phase record
// under its own label (never clobbering the build phase's own records).
func TestRemediation_RecordsFixDispatchInPhaseRecord(t *testing.T) {
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

// diagRunner returns a FAIL with error-severity diagnostics — the audit
// in-process override shape (cycle-1022).
type diagRunner struct{ name Phase }

func (r *diagRunner) Name() string { return string(r.name) }
func (r *diagRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: string(r.name), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace,
		Diagnostics: []Diagnostic{{Severity: "error", Message: "apicover -enforce flagged 1 line(s) — unnamed export"}}}, nil
}

// TestFailReasonsSurfaceInResult pins the cycle-1022 lesson: a floor-override
// FAIL's explanation must reach the RESULT (summary + dossier surfaces), not
// just workspace artifacts and orchestrator memory.
func TestFailReasonsSurfaceInResult(t *testing.T) {
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

// TestDispatch_ReadOnlyPhasesAreFencedAndSourceWritersAreNot is the core half
// of the worktree fence (ADR-0097): every dispatched request carries
// WorktreeReadOnly derived from the ONE write-permission predicate
// (worktreePhase) — read-only phases (scout, audit) true, the declared source
// writers (tdd, build) false — and a remediation builder fix never inherits
// the fenced gate's flag (a fenced builder would have its fix silently
// undone). Uses scout as the remediable read-only gate so the inherited-flag
// path is actually exercised.
func TestDispatch_ReadOnlyPhasesAreFencedAndSourceWritersAreNot(t *testing.T) {
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

// TestDispatch_ReadOnlyFlagOnResumeAndEvaluateBatchSurfaces — the other two
// request builders derive the same flag: the crash-resume path and the
// parallel-evaluate batch. A flag wired on the live loop alone would leave
// these red.
func TestDispatch_ReadOnlyFlagOnResumeAndEvaluateBatchSurfaces(t *testing.T) {
	// Resume from audit: the resumed audit is fenced, the ship after it is a
	// native phase (no worktree writes to fence — and it is not a source writer).
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

	// Evaluate batch: read-only evaluate phases are fenced through phaseRequestFor.
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

package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// recordingReviewer is a DeliverableReviewer test double that records every
// Review call and returns a scripted decision per phase. nil entry = approve.
type recordingReviewer struct {
	calls    []ReviewInput
	decide   map[string]ReviewResult
	reason   string
	mu       []ReviewInput // history of inputs in call order
	default_ ReviewResult
}

func (r *recordingReviewer) Review(_ context.Context, in ReviewInput) ReviewResult {
	r.calls = append(r.calls, in)
	if d, ok := r.decide[in.Phase]; ok {
		return d
	}
	return r.default_
}

// TestOrchestrator_NoopReviewer_IsByteIdentical is the WS-E2 pre-opt-in
// contract: when the operator doesn't pass WithReviewer, the orchestrator
// runs every phase to completion exactly like pre-E2 — no review-gate behavior
// observable.
func TestOrchestrator_NoopReviewer_IsByteIdentical(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 9}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	// Explicitly NO WithReviewer call.

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p",
		GoalHash:    "g",
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	// Cycle ran all phases — the noopReviewer didn't block anything.
	want := []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip}
	if len(res.PhasesRun) != len(want) {
		t.Errorf("PhasesRun=%v, want %v", res.PhasesRun, want)
	}
}

// TestOrchestrator_ReviewerApproves_CycleAdvances proves the positive path:
// an injected reviewer that returns Approve:true on every phase doesn't
// disturb the cycle, AND the reviewer was actually consulted (calls > 0).
func TestOrchestrator_ReviewerApproves_CycleAdvances(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &recordingReviewer{default_: ReviewResult{Approve: true}}
	o := NewOrchestrator(st, led, buildRunners(nil), WithReviewer(rev))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	// Reviewer was called for every non-SKIPPED phase that ran.
	if len(rev.calls) == 0 {
		t.Error("reviewer was never consulted despite WithReviewer being set")
	}
	// Each call must include the phase identity + workspace.
	for _, c := range rev.calls {
		if c.Phase == "" {
			t.Errorf("ReviewInput.Phase is empty: %+v", c)
		}
		if c.ProjectRoot == "" {
			t.Errorf("ReviewInput.ProjectRoot is empty: %+v", c)
		}
	}
}

// TestOrchestrator_ReviewerRejects_CycleAborts is the core E2 invariant: a
// reviewer that returns Approve:false on a specific phase ultimately aborts the
// cycle without recording that phase as a success, surfacing the reason in the
// error. With the default EVOLVE_CONTRACT_CORRECTION_RETRIES (2) and a reviewer
// that rejects every time, this drives the correction-exhaustion path (2
// re-dispatches, all still rejected) before the abort. The immediate-abort
// (retries=0) and reject-then-approve paths are pinned by the TestCorrectionLoop_*
// tests.
func TestOrchestrator_ReviewerRejects_CycleAborts(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &recordingReviewer{
		default_: ReviewResult{Approve: true},
		decide: map[string]ReviewResult{
			"build": {Approve: false, Reason: "deliverable missing required header"},
		},
	}
	o := NewOrchestrator(st, led, buildRunners(nil), WithReviewer(rev))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"})
	if err == nil {
		t.Fatal("expected RunCycle to abort on reviewer rejection; got nil")
	}
	if !strings.Contains(err.Error(), "review gate") {
		t.Errorf("error should mention review gate; got %v", err)
	}
	if !strings.Contains(err.Error(), "build") {
		t.Errorf("error should name the rejected phase; got %v", err)
	}
	if !strings.Contains(err.Error(), "deliverable missing required header") {
		t.Errorf("error should surface the reviewer's Reason; got %v", err)
	}
	// Ledger entries must NOT include a build SUCCESS — the reject runs
	// BEFORE the success ledger append for that phase. (Contract-correction
	// re-dispatches DO append "contract_correction" entries with the phase
	// Role; those are not successes, so the intent is Kind=="phase".)
	for _, e := range led.entries {
		if e.Role == "build" && e.Kind == "phase" {
			t.Errorf("rejected phase 'build' was recorded as a success in the ledger: %+v", e)
		}
	}
}

// sequencedReviewer returns results[i] on the i-th Review of `phase`; once the
// slice is exhausted it returns the last element. Other phases approve. Used to
// script reject→approve sequences for the contract-correction loop.
type sequencedReviewer struct {
	phase   string
	results []ReviewResult
	calls   int
}

func (s *sequencedReviewer) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != s.phase {
		return ReviewResult{Approve: true}
	}
	i := s.calls
	s.calls++
	if i >= len(s.results) {
		i = len(s.results) - 1
	}
	return s.results[i]
}

// TestCorrectionLoop_RejectThenApprove: one reject then approve ⇒ exactly one
// correction re-dispatch carrying the violation directive, then the cycle
// proceeds normally.
func TestCorrectionLoop_RejectThenApprove(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{
		{Approve: false, Reason: "deliverable missing required header"},
		{Approve: true},
	}}
	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	auditR := runners[PhaseAudit].(*fakeRunner)
	o := NewOrchestrator(st, led, runners, WithReviewer(rev))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle should proceed after one correction; got %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	if rev.calls != 2 {
		t.Errorf("build reviewed %d times, want 2 (initial + 1 re-review)", rev.calls)
	}
	if buildR.calls != 2 {
		t.Errorf("build runner ran %d times, want 2 (initial + 1 correction re-dispatch)", buildR.calls)
	}
	if len(buildR.requests) < 2 {
		t.Fatalf("expected >=2 build requests, got %d", len(buildR.requests))
	}
	cd := buildR.requests[1].CorrectionDirective
	if cd == "" || !strings.Contains(cd, "deliverable missing required header") {
		t.Errorf("correction re-dispatch directive missing/incomplete: %q", cd)
	}
	// The directive must not leak into a later phase's request.
	if len(auditR.requests) > 0 && auditR.requests[0].CorrectionDirective != "" {
		t.Errorf("CorrectionDirective leaked into audit: %q", auditR.requests[0].CorrectionDirective)
	}
	// A contract_correction ledger entry was recorded for observability.
	var sawCorr bool
	for _, e := range led.entries {
		if e.Role == "build" && e.Kind == "contract_correction" {
			sawCorr = true
		}
	}
	if !sawCorr {
		t.Error("expected a contract_correction ledger entry for the build re-dispatch")
	}
}

// TestCorrectionLoop_ExhaustsThenAborts: always reject ⇒ abort after the
// default 2 corrections, error names the count and the reason.
func TestCorrectionLoop_ExhaustsThenAborts(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &recordingReviewer{
		default_: ReviewResult{Approve: true},
		decide:   map[string]ReviewResult{"build": {Approve: false, Reason: "still malformed"}},
	}
	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	o := NewOrchestrator(st, led, runners, WithReviewer(rev))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"})
	if err == nil {
		t.Fatal("expected abort after corrections exhausted; got nil")
	}
	if !strings.Contains(err.Error(), "after 2 correction") {
		t.Errorf("error should name the correction count; got %v", err)
	}
	if !strings.Contains(err.Error(), "still malformed") {
		t.Errorf("error should surface the reviewer reason; got %v", err)
	}
	// initial dispatch + 2 correction re-dispatches = 3 runs.
	if buildR.calls != 3 {
		t.Errorf("build runner ran %d times, want 3 (initial + 2 corrections)", buildR.calls)
	}
}

// sequencedRunner returns verdicts[i] on the i-th Run (clamped to the last
// element), with no error. Used to script a correction re-dispatch that returns
// a non-canonical verdict.
type sequencedRunner struct {
	name     string
	verdicts []string
	calls    int
}

func (r *sequencedRunner) Name() string { return r.name }
func (r *sequencedRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	v := r.verdicts[min(r.calls, len(r.verdicts)-1)]
	r.calls++
	return PhaseResponse{Phase: r.name, Verdict: v, ArtifactsDir: req.Workspace}, nil
}

// TestCorrectionLoop_NonCanonicalVerdictAborts: a correction re-dispatch that
// returns err==nil but a non-canonical verdict aborts (same invariant the outer
// attempt loop enforces) rather than slipping a bad verdict downstream.
func TestCorrectionLoop_NonCanonicalVerdictAborts(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{
		{Approve: false, Reason: "missing header"},
		{Approve: true}, // never reached — the bad verdict aborts first
	}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = &sequencedRunner{name: "build", verdicts: []string{VerdictPASS, ""}}
	o := NewOrchestrator(st, led, runners, WithReviewer(rev))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"})
	if err == nil {
		t.Fatal("expected abort on a non-canonical correction verdict; got nil")
	}
	if !strings.Contains(err.Error(), "non-canonical verdict") {
		t.Errorf("error should name the non-canonical verdict; got %v", err)
	}
}

// TestCorrectionLoop_DisabledIsImmediateAbort: =0 ⇒ no re-dispatch, immediate
// abort with the pre-feature error message (byte-identical disable path).
func TestCorrectionLoop_DisabledIsImmediateAbort(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &recordingReviewer{
		default_: ReviewResult{Approve: true},
		decide:   map[string]ReviewResult{"build": {Approve: false, Reason: "x"}},
	}
	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	retryCfg := policy.Policy{}.RetryConfig()
	retryCfg.ContractCorrectionRetries = 0
	o := NewOrchestrator(st, led, runners, WithReviewer(rev), WithRetryConfig(retryCfg))

	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p",
		GoalHash:    "g",
		Env:         map[string]string{},
	})
	if err == nil {
		t.Fatal("expected immediate abort when corrections disabled; got nil")
	}
	if strings.Contains(err.Error(), "correction") {
		t.Errorf("disabled path must use the pre-feature message (no correction count); got %v", err)
	}
	if !strings.Contains(err.Error(), "deliverable rejected:") {
		t.Errorf("disabled path should keep the original abort message; got %v", err)
	}
	if buildR.calls != 1 {
		t.Errorf("build runner ran %d times, want 1 (no re-dispatch when disabled)", buildR.calls)
	}
}

// TestOrchestrator_ReviewerSkippedPhasesNotConsulted pins the SKIPPED-bypass
// rule: a phase that returns VerdictSKIPPED produced no deliverable to
// review, so the reviewer is NOT called for it (avoiding noise + false
// rejections of skipped-by-policy phases).
func TestOrchestrator_ReviewerSkippedPhasesNotConsulted(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &recordingReviewer{default_: ReviewResult{Approve: true}}
	// Build runners; pin build-planner to SKIPPED (it's the typical skip in
	// the canonical cycle when EVOLVE_BUILD_PLANNER=0).
	runners := buildRunners(map[Phase]string{PhaseBuildPlanner: VerdictSKIPPED})
	o := NewOrchestrator(st, led, runners, WithReviewer(rev))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	// No Review call should have phase == "build-planner".
	for _, c := range rev.calls {
		if c.Phase == "build-planner" {
			t.Errorf("reviewer was consulted for a SKIPPED phase: %+v", c)
		}
	}
}

// TestNoopReviewer_AlwaysApproves is the unit-level pin on the noopReviewer
// default — every input shape gets Approve:true.
func TestNoopReviewer_AlwaysApproves(t *testing.T) {
	t.Parallel()
	r := noopReviewer{}
	for _, phase := range []string{"scout", "tdd", "build", "ship", "user-defined-phase"} {
		got := r.Review(context.Background(), ReviewInput{Phase: phase})
		if !got.Approve {
			t.Errorf("noopReviewer.Review(%q).Approve=false, want true", phase)
		}
	}
}

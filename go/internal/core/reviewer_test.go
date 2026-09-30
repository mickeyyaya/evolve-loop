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

func TestOrchestrator_NoopReviewer_IsByteIdentical(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 9}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "g",
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	want := []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip}
	if len(res.PhasesRun) != len(want) {
		t.Errorf("PhasesRun=%v, want %v", res.PhasesRun, want)
	}
}

func TestOrchestrator_ReviewerApproves_CycleAdvances(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &recordingReviewer{default_: ReviewResult{Approve: true}}
	o := NewOrchestrator(st, led, buildRunners(nil), WithReviewer(rev))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	if len(rev.calls) == 0 {
		t.Error("reviewer was never consulted despite WithReviewer being set")
	}
	for _, c := range rev.calls {
		if c.Phase == "" {
			t.Errorf("ReviewInput.Phase is empty: %+v", c)
		}
		if c.ProjectRoot == "" {
			t.Errorf("ReviewInput.ProjectRoot is empty: %+v", c)
		}
	}
}

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

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
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
	// Kind=="phase" marks a success append; a contract-correction re-dispatch
	// appends Kind=="contract_correction" instead, so this checks for a
	// build SUCCESS specifically, not just any build-role entry.
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

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
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
	if len(auditR.requests) > 0 && auditR.requests[0].CorrectionDirective != "" {
		t.Errorf("CorrectionDirective leaked into audit: %q", auditR.requests[0].CorrectionDirective)
	}
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

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err == nil {
		t.Fatal("expected abort after corrections exhausted; got nil")
	}
	if !strings.Contains(err.Error(), "after 2 correction") {
		t.Errorf("error should name the correction count; got %v", err)
	}
	if !strings.Contains(err.Error(), "still malformed") {
		t.Errorf("error should surface the reviewer reason; got %v", err)
	}
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

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err == nil {
		t.Fatal("expected abort on a non-canonical correction verdict; got nil")
	}
	if !strings.Contains(err.Error(), "non-canonical verdict") {
		t.Errorf("error should name the non-canonical verdict; got %v", err)
	}
}

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
		ProjectRoot: t.TempDir(),
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

func TestOrchestrator_ReviewerSkippedPhasesNotConsulted(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	rev := &recordingReviewer{default_: ReviewResult{Approve: true}}
	runners := buildRunners(map[Phase]string{PhaseBuildPlanner: VerdictSKIPPED})
	o := NewOrchestrator(st, led, runners, WithReviewer(rev))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	for _, c := range rev.calls {
		if c.Phase == "build-planner" {
			t.Errorf("reviewer was consulted for a SKIPPED phase: %+v", c)
		}
	}
}

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

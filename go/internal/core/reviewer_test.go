package core

import (
	"context"
	"strings"
	"testing"
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
// reviewer that returns Approve:false on a specific phase aborts the cycle
// BEFORE the ledger append for that phase, surfacing the reason in the error.
func TestOrchestrator_ReviewerRejects_CycleAborts(t *testing.T) {
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
	// Ledger entries must NOT include build — the reject runs BEFORE the
	// ledger append for that phase, so the rejected phase doesn't get
	// recorded as a success.
	for _, e := range led.entries {
		if e.Role == "build" {
			t.Errorf("rejected phase 'build' was appended to the ledger: %+v", e)
		}
	}
}

// TestOrchestrator_ReviewerSkippedPhasesNotConsulted pins the SKIPPED-bypass
// rule: a phase that returns VerdictSKIPPED produced no deliverable to
// review, so the reviewer is NOT called for it (avoiding noise + false
// rejections of skipped-by-policy phases).
func TestOrchestrator_ReviewerSkippedPhasesNotConsulted(t *testing.T) {
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
	r := noopReviewer{}
	for _, phase := range []string{"scout", "tdd", "build", "ship", "user-defined-phase"} {
		got := r.Review(context.Background(), ReviewInput{Phase: phase})
		if !got.Approve {
			t.Errorf("noopReviewer.Review(%q).Approve=false, want true", phase)
		}
	}
}

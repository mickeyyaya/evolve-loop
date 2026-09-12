package core

// remediation_rerun_review_test.go — ADR-0100 §4: the remediated gate re-run's
// deliverable goes through the same review as the original dispatch.
//
// maybeRemediate re-dispatches the failed gate after the Builder's fix and
// overwrote dr.resp with the re-run's response; that response then reached
// recordAndBranch without ever meeting the reviewer, so a re-run that omitted
// a declared deliverable — or any contract check — was recorded on the
// strength of its verdict alone.

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// phaseReviewCounter counts Review calls per phase and approves everything.
type phaseReviewCounter struct{ byPhase map[string]int }

func (c *phaseReviewCounter) Review(_ context.Context, in ReviewInput) ReviewResult {
	if c.byPhase == nil {
		c.byPhase = map[string]int{}
	}
	c.byPhase[in.Phase]++
	return ReviewResult{Approve: true}
}

func TestRemediation_GateReRunIsReviewed(t *testing.T) {
	runners := buildRunners(nil)
	gate := &scriptedRunner{name: PhaseTDD, verdicts: []string{VerdictFAIL, VerdictPASS}}
	build := &scriptedRunner{name: PhaseBuild, verdicts: []string{VerdictPASS}}
	runners[PhaseTDD], runners[PhaseBuild] = gate, build
	reviews := &phaseReviewCounter{}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorkflowConfig(policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}),
		WithReviewer(reviews))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if gate.calls != 2 {
		t.Fatalf("precondition: the gate must run twice (FAIL, then the post-fix re-run), got %d", gate.calls)
	}
	if got := reviews.byPhase[string(PhaseTDD)]; got != 2 {
		t.Fatalf("tdd reviewed %d time(s), want 2 — the remediated re-run's deliverable reached recordAndBranch unreviewed", got)
	}
}

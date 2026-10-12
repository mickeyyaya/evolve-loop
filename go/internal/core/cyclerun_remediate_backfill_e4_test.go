package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type remediateReviewerE4 struct {
	decide map[string]ReviewResult
	seen   []string
}

func (r *remediateReviewerE4) Review(_ context.Context, in ReviewInput) ReviewResult {
	r.seen = append(r.seen, in.Phase)
	if res, ok := r.decide[in.Phase]; ok {
		return res
	}
	return ReviewResult{Approve: true}
}

type remediateRigStateE4 struct {
	cr    *cycleRun
	dr    *dispatchResult
	gate  *scriptedRunner
	build *scriptedRunner
}

func remediateRigE4(t *testing.T, rounds int, reviewer DeliverableReviewer, gateVerdicts []string) remediateRigStateE4 {
	t.Helper()
	gate := &scriptedRunner{name: PhaseTDD, verdicts: gateVerdicts}
	build := &scriptedRunner{name: PhaseBuild, verdicts: []string{VerdictPASS}}
	runners := buildRunners(nil)
	runners[PhaseTDD], runners[PhaseBuild] = gate, build
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorkflowConfig(policy.WorkflowConfig{RemediationRounds: rounds, RemediablePhases: []string{"tdd"}}))
	if reviewer != nil {
		o.reviewer = reviewer
	}
	cr := &cycleRun{
		o: o, ctx: context.Background(), cycle: 4, req: CycleRequest{ProjectRoot: t.TempDir()},
		cs:          CycleState{CycleID: 4, RunID: "run-4", WorkspacePath: t.TempDir()},
		retryConfig: o.retryConfig, workflowConfig: o.workflowConfig, envSnap: map[string]string{}, ctxSnap: map[string]string{},
	}
	dr := &dispatchResult{resp: PhaseResponse{Phase: "tdd", Verdict: VerdictFAIL}, attemptCount: 1, runner: gate}
	return remediateRigStateE4{cr: cr, dr: dr, gate: gate, build: build}
}

func TestMaybeRemediate_ALaneWithoutABuildRunnerLeavesTheGateFailStanding(t *testing.T) {
	t.Parallel()

	rig := remediateRigE4(t, 1, nil, []string{VerdictPASS})
	delete(rig.cr.o.runners, PhaseBuild)

	act, err := rig.cr.maybeRemediate(PhaseTDD, rig.dr)

	if act != loopNext || err != nil || rig.dr.resp.Verdict != VerdictFAIL {
		t.Fatalf("act=%v err=%v verdict=%q, want the FAIL to stand", act, err, rig.dr.resp.Verdict)
	}
	if rig.gate.calls != 0 || len(rig.cr.result.Remediations) != 0 || rig.cr.remediationRounds != nil {
		t.Fatalf("no round may start without a builder: gate=%d rems=%v rounds=%v", rig.gate.calls, rig.cr.result.Remediations, rig.cr.remediationRounds)
	}
}

func TestMaybeRemediate_ASpentRoundBudgetNeverDispatchesAgain(t *testing.T) {
	t.Parallel()

	rig := remediateRigE4(t, 2, nil, []string{VerdictPASS})
	rig.cr.remediationRounds = map[Phase]int{PhaseTDD: 2}

	act, err := rig.cr.maybeRemediate(PhaseTDD, rig.dr)

	if act != loopNext || err != nil || rig.build.calls != 0 || rig.gate.calls != 0 {
		t.Fatalf("act=%v err=%v build=%d gate=%d, want no dispatch at the cap", act, err, rig.build.calls, rig.gate.calls)
	}
	if rig.cr.remediationRounds[PhaseTDD] != 2 || len(rig.cr.result.Remediations) != 0 {
		t.Fatalf("rounds=%v rems=%v, the counter must not move past the cap", rig.cr.remediationRounds, rig.cr.result.Remediations)
	}
}

func TestMaybeRemediate_ARoundBelowTheCapStillRunsAndCountsItself(t *testing.T) {
	t.Parallel()

	rig := remediateRigE4(t, 2, nil, []string{VerdictPASS})
	rig.cr.remediationRounds = map[Phase]int{PhaseTDD: 1}

	act, err := rig.cr.maybeRemediate(PhaseTDD, rig.dr)

	if act != loopNext || err != nil || rig.build.calls != 1 || rig.gate.calls != 1 || rig.cr.remediationRounds[PhaseTDD] != 2 {
		t.Fatalf("act=%v err=%v build=%d gate=%d rounds=%v, want round 2 of 2 to run", act, err, rig.build.calls, rig.gate.calls, rig.cr.remediationRounds)
	}
	if rig.dr.resp.Verdict != VerdictPASS {
		t.Fatalf("the re-run verdict must replace the FAIL; got %q", rig.dr.resp.Verdict)
	}
}

func TestMaybeRemediate_ARejectedBuilderFixVoidsTheRoundAndTheGateFailStands(t *testing.T) {
	reviewer := &remediateReviewerE4{decide: map[string]ReviewResult{"build": {Approve: false, Reason: "builder touched unrelated files"}}}
	rig := remediateRigE4(t, 1, reviewer, []string{VerdictPASS})

	var act loopAction
	var err error
	log := captureStderr(t, func() { act, err = rig.cr.maybeRemediate(PhaseTDD, rig.dr) })

	if act != loopNext || err != nil {
		t.Fatalf("act=%v err=%v, a voided round never aborts the cycle", act, err)
	}
	if rig.gate.calls != 0 || rig.dr.resp.Verdict != VerdictFAIL {
		t.Fatalf("gate re-ran %d times with verdict %q; a rejected fix must not re-verify", rig.gate.calls, rig.dr.resp.Verdict)
	}
	if len(rig.cr.result.Remediations) != 1 || !strings.Contains(rig.cr.result.Remediations[0], "tdd: round 1 voided (Build review rejected:") {
		t.Fatalf("provenance = %v", rig.cr.result.Remediations)
	}
	if !strings.Contains(log, "WARN remediation: tdd: round 1 voided") || !strings.Contains(log, "gate FAIL stands") {
		t.Fatalf("the void must be loud:\n%s", log)
	}
}

func TestMaybeRemediate_AResealedExplanationThatCannotBeProjectedVoidsTheRound(t *testing.T) {
	rig := remediateRigE4(t, 1, &remediateReviewerE4{}, []string{VerdictPASS})
	rig.cr.cs.ExplanationDocumentationVersion = explanationdocs.CurrentContractVersion

	var act loopAction
	var err error
	log := captureStderr(t, func() { act, err = rig.cr.maybeRemediate(PhaseTDD, rig.dr) })

	if act != loopNext || err != nil || rig.gate.calls != 0 || rig.dr.resp.Verdict != VerdictFAIL {
		t.Fatalf("act=%v err=%v gate=%d verdict=%q, want the round voided before the re-run", act, err, rig.gate.calls, rig.dr.resp.Verdict)
	}
	if len(rig.cr.result.Remediations) != 1 || !strings.Contains(rig.cr.result.Remediations[0], "tdd: round 1 voided (resealed Build explanation unavailable:") {
		t.Fatalf("provenance = %v", rig.cr.result.Remediations)
	}
	if !strings.Contains(log, "WARN remediation: tdd: round 1 voided (resealed Build explanation unavailable") {
		t.Fatalf("the void must be loud:\n%s", log)
	}
}

func TestMaybeRemediate_AReRunThatTheReviewerRejectsAbortsTheCycle(t *testing.T) {
	reviewer := &remediateReviewerE4{decide: map[string]ReviewResult{"tdd": {Approve: false, Reason: "re-run omitted the declared deliverable"}}}
	rig := remediateRigE4(t, 1, reviewer, []string{VerdictPASS})

	var act loopAction
	var err error
	captureStderr(t, func() { act, err = rig.cr.maybeRemediate(PhaseTDD, rig.dr) })

	if act != loopAbort || err == nil {
		t.Fatalf("act=%v err=%v, want the re-run's rejection to abort", act, err)
	}
	if rig.build.calls != 1 {
		t.Fatalf("the builder fix must have been approved and dispatched once; build=%d", rig.build.calls)
	}
	if len(reviewer.seen) < 2 || reviewer.seen[0] != "build" || reviewer.seen[1] != "tdd" {
		t.Fatalf("review order = %v, want the build fix reviewed before the gate re-run", reviewer.seen)
	}
}

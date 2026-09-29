package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

type codedTriageRunner struct {
	triageDecisionRunner
	diags []Diagnostic
}

func (r codedTriageRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	resp, err := r.triageDecisionRunner.Run(ctx, req)
	resp.Diagnostics = r.diags
	return resp, err
}

func lastTimingEntry(t *testing.T, workspace string) phasetiming.Entry {
	t.Helper()
	entries, err := phasetiming.Read(workspace)
	if err != nil || len(entries) == 0 {
		t.Fatalf("phase-timing.json: %v (%d entries)", err, len(entries))
	}
	return entries[len(entries)-1]
}

func laneCycleWorkspace(t *testing.T, triage PhaseRunner) (CycleResult, string) {
	t.Helper()
	var root string
	result, _ := runLaneCycleWith(t, triage, func(t *testing.T, r string) { root = r })
	return result, cycleWorkspaceDir(root, result.Cycle)
}

func sequentialCycleWorkspace(t *testing.T, triage PhaseRunner) (CycleResult, string) {
	t.Helper()
	root := writeClaimableInbox(t, 1)
	runners := buildRunners(nil)
	runners[PhaseTriage] = triage
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))
	result, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "claimable-work"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	return result, cycleWorkspaceDir(root, result.Cycle)
}

func TestCompleteCycle_AClaimFailedLaneRecordsItsUnansweredScopeAtC1(t *testing.T) {
	t.Parallel()
	result, ws := laneCycleWorkspace(t, triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[]}`})

	e := lastTimingEntry(t, ws)
	if e.Phase != string(PhaseTriage) || e.Verdict != VerdictFAIL || e.AbortReason != cycleTerminationTriageClaimFailed {
		t.Fatalf("last C1 entry = %+v; the claim-failed ending is recorded at the chokepoint with its reason", e)
	}
	if codes := cyclestate.ErrorCodes(e.Diagnostics); len(codes) != 1 || codes[0] != cyclestate.DiagCodeTriageScopeUnanswered || e.Diagnostics[0].Subject != laneScopedItem {
		t.Errorf("diagnostics = %+v; the lane's unanswered pin is named", e.Diagnostics)
	}
	if cls := cycleclassify.Classify(ws); cls.Class != cycleclassify.ClassPhaseRefusal || cls.Marker != cyclestate.DiagCodeTriageScopeUnanswered {
		t.Errorf("Classify = %+v; cycle 1757's ending is the lane's task-level refusal, never integrity-breach", cls)
	}
	if oc, _ := cyclehealth.ClassifyOutcome(ws); oc != cyclehealth.OutcomeFailedExplained {
		t.Errorf("outcome = %s; the recorded reason explains the FAIL", oc)
	}
	if result.FinalVerdict != VerdictFAIL {
		t.Errorf("FinalVerdict = %q", result.FinalVerdict)
	}
}

func TestCompleteCycle_APlannedNoWorkEndIsRecordedAtC1AsNoWork(t *testing.T) {
	t.Parallel()
	_, ws := laneCycleWorkspace(t, triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[],"escalate_block":[{"task_id":"claimable-lane-item","reason":"console-routed"}]}`})

	e := lastTimingEntry(t, ws)
	if e.Phase != string(PhaseTriage) || e.Verdict != VerdictSKIPPED || e.AbortReason != CycleTerminationTriageNoWork {
		t.Fatalf("last C1 entry = %+v; the planned no-work ending is recorded with its reason", e)
	}
	if oc, _ := cyclehealth.ClassifyOutcome(ws); oc != cyclehealth.OutcomeNoWork {
		t.Errorf("outcome = %s; cycle 1758's planned end is NO_WORK, never the FAILED_UNEXPLAINED alarm", oc)
	}
}

func TestCompleteCycle_ASequentialClaimFailureKeepsItsClassification(t *testing.T) {
	t.Parallel()
	_, ws := sequentialCycleWorkspace(t, triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[]}`})

	e := lastTimingEntry(t, ws)
	if e.AbortReason != cycleTerminationTriageClaimFailed || len(cyclestate.ErrorCodes(e.Diagnostics)) != 0 {
		t.Fatalf("last C1 entry = %+v; a sequential cycle has no pin to name, so its record carries the reason and no code", e)
	}
	if cls := cycleclassify.Classify(ws); cls.Class == cycleclassify.ClassPhaseRefusal {
		t.Errorf("Classify = %+v; without a lane pin the sequential batch keeps its halt classification", cls)
	}
	if oc, _ := cyclehealth.ClassifyOutcome(ws); oc != cyclehealth.OutcomeFailedExplained {
		t.Errorf("outcome = %s; the recorded reason explains the FAIL", oc)
	}
}

func TestCompleteCycle_TriagesOwnRefusalStaysFirstOnTheClaimFailedRecord(t *testing.T) {
	t.Parallel()
	own := Diagnostic{Severity: cyclestate.SeverityError, Code: cyclestate.DiagCodeTriageTopNEmpty, Message: "## top_n section has no list items"}
	_, ws := sequentialCycleWorkspace(t, codedTriageRunner{triageDecisionRunner: triageDecisionRunner{verdict: VerdictFAIL, decision: `{"top_n":[]}`}, diags: []Diagnostic{own}})

	if cls := cycleclassify.Classify(ws); cls.Class != cycleclassify.ClassPhaseRefusal || cls.Marker != cyclestate.DiagCodeTriageTopNEmpty {
		t.Errorf("Classify = %+v; the host's closeout record never shadows triage's own coded refusal", cls)
	}
}

func TestCompleteCycle_ALanesOwnTriageRefusalOutranksItsScopeCode(t *testing.T) {
	t.Parallel()
	own := Diagnostic{Severity: cyclestate.SeverityError, Code: cyclestate.DiagCodeTriageTopNEmpty, Message: "## top_n section has no list items"}
	_, ws := laneCycleWorkspace(t, codedTriageRunner{triageDecisionRunner: triageDecisionRunner{verdict: VerdictFAIL, decision: `{"top_n":[]}`}, diags: []Diagnostic{own}})

	codes := cyclestate.ErrorCodes(lastTimingEntry(t, ws).Diagnostics)
	if len(codes) != 2 || codes[0] != cyclestate.DiagCodeTriageTopNEmpty || codes[1] != cyclestate.DiagCodeTriageScopeUnanswered {
		t.Fatalf("codes = %v; triage's own refusal first, then the lane's unanswered scope", codes)
	}
	if cls := cycleclassify.Classify(ws); cls.Marker != cyclestate.DiagCodeTriageTopNEmpty {
		t.Errorf("Classify = %+v; the host's scope code never shadows triage's own refusal", cls)
	}
}

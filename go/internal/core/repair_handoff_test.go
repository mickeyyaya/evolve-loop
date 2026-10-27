package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	handoffItem    = "retro-minting-files-through-inbox-add-lane-ac-5-console"
	handoffSalvage = "/dev/cl-c2035-lane"
	handoffDefect  = "M1: go/internal/core/failurelearning floor tests fail (3) with this diff; protected surface, fix is console item " + handoffItem
	laneDefect     = "H1: go/internal/core/retro_remediation_filter_test.go:29 inboxFiles counts .bump.lock; unprotected, lane-fixable"
)

type stubRepairCheck struct {
	give  RepairFindings
	calls []RepairCheck
}

func (s *stubRepairCheck) check(in RepairCheck) RepairFindings {
	s.calls = append(s.calls, in)
	return s.give
}

var (
	handoffStop = RepairFindings{Stop: true, Reason: "handoff_blocker", Handoff: []string{"M1"}, Blockers: []string{"M1"}, ConsoleItems: []string{handoffItem}, Salvage: handoffSalvage}
	handoffNone = RepairFindings{Handoff: []string{"M2"}, Lane: []string{"H1"}}
	laneOnly    = RepairFindings{Lane: []string{"H1"}}
)

func handoffWorkspace(t *testing.T, defects ...string) string {
	t.Helper()
	ws := filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-2035")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	fb := phasecontract.FailureBlock{Class: "code-audit-warn", Defects: defects}
	report := "# Audit\n\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", &fb) + "\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func handoffOrchestrator(stage string, give RepairFindings) (*Orchestrator, *stubRepairCheck, *[]signalcenter.Event) {
	wf := policy.Policy{}.WorkflowConfig()
	wf.Repair.Fixability.Stage = stage
	center, events := captureSignals()
	stub := &stubRepairCheck{give: give}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(wf), WithSignalCenter(center), WithRepairFixability(stub.check))
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	return o, stub, events
}

func handoffState(t *testing.T) CycleState {
	t.Helper()
	return CycleState{CycleID: 2035, WorkspacePath: handoffWorkspace(t, handoffDefect), ActiveWorktree: handoffSalvage, AuditFailReasons: []string{"repo-contract RED"}, AuditDispatches: 1}
}

func TestDecideAfterAuditFail_AtEnforceAStopHandsOffWithNoRepairRound(t *testing.T) {
	t.Parallel()
	o, stub, events := handoffOrchestrator(policy.FixabilityEnforce, handoffStop)
	adjudicator := &stubAdjudicator{give: &adjudication{Action: retryActionRetryTDD}}
	o.retryAdjudicator = adjudicator
	cs := handoffState(t)
	if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "ship-error.json"), []byte(`{"class":"precondition","code":"REPO_CONTRACT_GATE","message":"backstop names go/internal/core/failurelearning"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	next, reason, sig := o.decideAfterAuditFail(cs)

	if next != PhaseRetro || sig != nil || !strings.HasPrefix(reason, repairHandoffReasonPrefix+"handoff_blocker") || adjudicator.consults != 0 {
		t.Fatalf("decision = %s %q %+v (consults %d), want retro with the hand-off reason and no repair round", next, reason, sig, adjudicator.consults)
	}
	if !strings.Contains(reason, "blockers M1") || !strings.Contains(reason, handoffItem) || !strings.Contains(reason, handoffSalvage) {
		t.Errorf("reason = %q, want M1, the console item and the salvage pointer", reason)
	}
	want := RepairCheck{
		Defects:      []string{handoffDefect},
		GateFailures: []string{"repo-contract RED", "backstop names go/internal/core/failurelearning"},
		InboxDir:     filepath.Join(filepath.Dir(filepath.Dir(cs.WorkspacePath)), "inbox"),
		Salvage:      handoffSalvage,
	}
	if len(stub.calls) != 1 || !reflect.DeepEqual(stub.calls[0], want) {
		t.Errorf("check input = %+v, want %+v", stub.calls, want)
	}
	got := eventsWithCode(*events, CodeRepairHandoff)
	if len(got) != 1 || got[0].Fields["decision"] != RepairDecisionStop || got[0].Fields["blockers"] != "M1" || got[0].Fields["stage"] != policy.FixabilityEnforce || got[0].Severity != signalcenter.SeverityWarn {
		t.Errorf("signals = %+v, want one REPAIR_HANDOFF stop on M1", got)
	}
	if len(eventsWithCode(*events, CodeAuditRepairDeclined)) != 1 {
		t.Error("the declined repair round must be recorded")
	}
	var written RepairFindings
	b, err := os.ReadFile(filepath.Join(cs.WorkspacePath, RepairHandoffFile))
	if err != nil || json.Unmarshal(b, &written) != nil || !reflect.DeepEqual(written, handoffStop) {
		t.Errorf("%s = %s, %v; want the stop decision with the salvage pointer", RepairHandoffFile, b, err)
	}
}

func TestDecideAfterAuditFail_AtShadowAStopRecordsWouldStopAndRepairsAsToday(t *testing.T) {
	t.Parallel()
	o, _, events := handoffOrchestrator(policy.FixabilityShadow, handoffStop)
	cs := handoffState(t)

	next, _, _ := o.decideAfterAuditFail(cs)

	got := eventsWithCode(*events, CodeRepairHandoff)
	if next != PhaseTDD || len(got) != 1 || got[0].Fields["decision"] != RepairDecisionWould {
		t.Fatalf("next %s, signals %+v; want today's repair round and one would_stop", next, got)
	}
	if _, err := os.Stat(filepath.Join(cs.WorkspacePath, RepairHandoffFile)); !os.IsNotExist(err) {
		t.Errorf("shadow wrote %s (%v); it must change no flow", RepairHandoffFile, err)
	}
}

func TestDecideAfterAuditFail_ANonBlockingHandoffFindingIsRecordedAndRepairs(t *testing.T) {
	t.Parallel()
	o, _, events := handoffOrchestrator(policy.FixabilityEnforce, handoffNone)

	next, _, _ := o.decideAfterAuditFail(handoffState(t))

	got := eventsWithCode(*events, CodeRepairHandoff)
	if next != PhaseTDD || len(got) != 1 || got[0].Fields["decision"] != RepairDecisionNone || got[0].Fields["handoff"] != "M2" || got[0].Fields["lane"] != "H1" || got[0].Severity != signalcenter.SeverityInfo {
		t.Errorf("next %s, signals %+v; want a repair round and one INFO none naming M2", next, got)
	}
}

func TestDecideAfterAuditFail_ALaneFindingAloneRecordsNothing(t *testing.T) {
	t.Parallel()
	o, _, events := handoffOrchestrator(policy.FixabilityEnforce, laneOnly)
	cs := CycleState{CycleID: 2035, WorkspacePath: handoffWorkspace(t, laneDefect)}

	if next, _, _ := o.decideAfterAuditFail(cs); next != PhaseTDD || len(eventsWithCode(*events, CodeRepairHandoff)) != 0 {
		t.Errorf("next %s, want a repair round and no signal without a hand-off finding", next)
	}
}

func TestDecideAfterAuditFail_NoCheckAtOffWithNoCheckOrWithNoRepairRound(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		stage    string
		attempts int
		unwired  bool
	}{
		"off":             {stage: policy.FixabilityOff},
		"no check":        {stage: policy.FixabilityEnforce, unwired: true},
		"budget is spent": {stage: policy.FixabilityEnforce, attempts: 2},
	} {
		o, stub, events := handoffOrchestrator(tc.stage, handoffStop)
		if tc.unwired {
			o.repairFixability = nil
		}
		cs := handoffState(t)
		cs.AuditRepairAttempts = tc.attempts

		_, reason, _ := o.decideAfterAuditFail(cs)

		if strings.HasPrefix(reason, repairHandoffReasonPrefix) || len(stub.calls) != 0 || len(eventsWithCode(*events, CodeRepairHandoff)) != 0 {
			t.Errorf("%s: reason %q, calls %d; want no fixability check", name, reason, len(stub.calls))
		}
	}
}

func TestRepairHandoff_AnUnwritableRecordWarnsAndStillStops(t *testing.T) {
	t.Parallel()
	o, _, _ := handoffOrchestrator(policy.FixabilityEnforce, handoffStop)
	var stderr strings.Builder
	o.stderr = &stderr
	cs := handoffState(t)
	if err := os.Mkdir(filepath.Join(cs.WorkspacePath, RepairHandoffFile), 0o755); err != nil {
		t.Fatal(err)
	}
	fb, _ := phasecontract.ReadFailureBlock(cs.WorkspacePath, string(PhaseAudit))

	_, stop := o.repairHandoff(cs, fb, retryEnvelope{Legal: []retryAction{retryActionRetryTDD}})

	if !stop || !strings.Contains(stderr.String(), RepairHandoffFile+" not written") {
		t.Errorf("stop %t, stderr %q; want the stop and a WARN", stop, stderr.String())
	}
}

func TestResumeAfterAudit_AHandoffKeepsTheTerminationOfTheResumedCycle(t *testing.T) {
	t.Parallel()
	o, _, _ := handoffOrchestrator(policy.FixabilityEnforce, handoffStop)
	cs := handoffState(t)
	cursor := newResumeCursor(PhaseAudit)
	var result CycleResult
	if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "triage-decision.json"), []byte(`{"top_n":[{"id":"retro-minting-files-through-inbox-add-lane"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	timings := []phaseTimingEntry{{Phase: "audit", Verdict: VerdictFAIL}}

	if err := o.resumeAfterAudit(&cs, &cursor, &result, timings, VerdictFAIL); err != nil {
		t.Fatal(err)
	}

	if cursor.termination.reason != CycleTerminationHandedOff || result.TerminationReason != CycleTerminationHandedOff || cursor.scheduledNext != "" {
		t.Errorf("cursor %+v result %q, want the handed-off termination and no scheduled repair", cursor, result.TerminationReason)
	}
	if len(timings[0].Diagnostics) != 1 || timings[0].Diagnostics[0].Code != cyclestate.DiagCodeHandoffRoute {
		t.Errorf("diagnostics = %+v, want the HANDOFF_ROUTE of the resumed hand-off", timings[0].Diagnostics)
	}
}

func TestSealHandedOff_SetsTheTerminationAndRoutesTheCommittedItem(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[{"id":"retro-minting-files-through-inbox-add-lane"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var result CycleResult
	earlier := Diagnostic{Code: "EARLIER"}
	timings := []phaseTimingEntry{{Phase: "audit", Verdict: VerdictFAIL, Diagnostics: []Diagnostic{earlier}}}
	cr := &cycleRun{o: &Orchestrator{}, cs: CycleState{WorkspacePath: ws}}

	sealed := cr.sealHandedOff(&result, timings, repairHandoffReasonPrefix+"handoff_blocker: blockers M1")

	if !sealed || result.TerminationReason != CycleTerminationHandedOff {
		t.Fatalf("sealed %t, result %+v", sealed, result)
	}
	d := timings[0].Diagnostics
	if len(d) != 2 || d[1].Code != earlier.Code || d[0].Code != cyclestate.DiagCodeHandoffRoute || d[0].Subject != "retro-minting-files-through-inbox-add-lane" || !strings.HasPrefix(d[0].Message, "handed off: handoff_blocker") {
		t.Errorf("diagnostics = %+v, want the HANDOFF_ROUTE for the committed item first", d)
	}
	result.FinalVerdict = VerdictFAIL
	if !IsHandedOffResult(result) || IsHandedOffResult(CycleResult{FinalVerdict: VerdictFAIL}) {
		t.Error("IsHandedOffResult must read the FAIL verdict and the handed-off termination")
	}
	if cr.sealHandedOff(&CycleResult{}, nil, auditDeclineReasonPrefix+"retry budget spent") {
		t.Error("an ordinary decline is not a hand-off")
	}
	unrouted := CycleResult{}
	two := []phaseTimingEntry{{Phase: "audit"}}
	center, events := captureSignals()
	multi := &cycleRun{o: &Orchestrator{signals: center}, cs: CycleState{WorkspacePath: t.TempDir()}}
	if !multi.sealHandedOff(&unrouted, two, repairHandoffReasonPrefix+"x") || len(two[0].Diagnostics) != 0 || len(eventsWithCode(*events, CodeBlockerRefused)) != 1 {
		t.Errorf("a cycle with no single committed item = %+v, events %d; want the seal, no route and one refusal signal", two, len(*events))
	}
}

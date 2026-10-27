package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/fixability"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	c2035AcFive  = "retro-minting-files-through-inbox-add-lane-ac-5-console"
	c2035Merge   = "retro-minting-files-through-inbox-add-console"
	c2035Salvage = "/dev/cl-c2035-lane"
)

type c2035Round struct {
	Failure      phasecontract.FailureBlock `json:"failure"`
	GateFailures []string                   `json:"gate_failures"`
}

func readC2035Round(t *testing.T, name string) c2035Round {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "fixability", "testdata", "cycle-2035", name))
	if err != nil {
		t.Fatal(err)
	}
	var r c2035Round
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func c2035Workspace(t *testing.T, r c2035Round) string {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-2035")
	inbox := filepath.Join(root, ".evolve", "inbox")
	for _, dir := range []string{ws, inbox} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	report := "# Audit\n\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", &r.Failure) + "\n"
	files := map[string]string{
		filepath.Join(ws, "audit-report.md"):      report,
		filepath.Join(inbox, c2035AcFive+".json"): `{"id":"` + c2035AcFive + `","weight":0.5,"route":"lane","files":["go/internal/core/failurelearning/floor_test.go"]}`,
		filepath.Join(inbox, c2035Merge+".json"):  `{"id":"` + c2035Merge + `","weight":0.5,"route":"console-manual"}`,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

func lifecycleFence(p string) bool {
	return strings.HasPrefix(p, "go/internal/inboxmover/lifecycle/") || strings.HasPrefix(p, "go/internal/core/failurelearning/")
}

func handoffOrchestrator(stage config.Stage) (*Orchestrator, *[]signalcenter.Event) {
	wf := policy.Policy{}.WorkflowConfig()
	wf.RepairFixability = stage
	center, events := captureSignals()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(wf), WithSignalCenter(center), WithProtectedSurface(lifecycleFence))
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	return o, events
}

func c2035State(t *testing.T, round string) CycleState {
	t.Helper()
	r := readC2035Round(t, round)
	return CycleState{CycleID: 2035, WorkspacePath: c2035Workspace(t, r), ActiveWorktree: c2035Salvage, AuditFailReasons: r.GateFailures, AuditDispatches: 1}
}

func TestDecideAfterAuditFail_Cycle2035AtEnforceHandsOffAtItsFirstAudit(t *testing.T) {
	t.Parallel()
	o, events := handoffOrchestrator(config.StageEnforce)
	stub := &stubAdjudicator{give: &adjudication{Action: retryActionRetryTDD}}
	o.retryAdjudicator = stub
	cs := c2035State(t, "round1.json")

	next, reason, sig := o.decideAfterAuditFail(cs)

	if next != PhaseRetro || sig != nil || !strings.HasPrefix(reason, repairHandoffReasonPrefix+fixability.ReasonHandoffBlocker) || stub.consults != 0 {
		t.Fatalf("decision = %s %q %+v (consults %d), want retro with the hand-off reason and no repair round", next, reason, sig, stub.consults)
	}
	if !strings.Contains(reason, "M1 (red_test)") || !strings.Contains(reason, c2035AcFive) || !strings.Contains(reason, c2035Salvage) {
		t.Errorf("reason = %q, want M1, the console item and the salvage pointer", reason)
	}
	got := eventsWithCode(*events, CodeRepairHandoff)
	if len(got) != 1 || got[0].Fields["decision"] != fixability.DecisionStop || got[0].Fields["blockers"] != "M1" || got[0].Fields["lane"] != "" || got[0].Severity != signalcenter.SeverityWarn {
		t.Errorf("signals = %+v, want one REPAIR_HANDOFF stop on M1", got)
	}
	if len(eventsWithCode(*events, CodeAuditRepairDeclined)) != 1 {
		t.Error("the declined repair round must be recorded")
	}
	var written fixability.Decision
	b, err := os.ReadFile(filepath.Join(cs.WorkspacePath, RepairHandoffFile))
	if err != nil || json.Unmarshal(b, &written) != nil || !written.Stop || written.Salvage != c2035Salvage {
		t.Errorf("%s = %s, %v; want the stop decision with the salvage pointer", RepairHandoffFile, b, err)
	}
}

func TestDecideAfterAuditFail_Cycle2035AtShadowRecordsWouldStopAndRepairsAsToday(t *testing.T) {
	t.Parallel()
	o, events := handoffOrchestrator(config.StageShadow)
	cs := c2035State(t, "round3.json")
	cs.AuditDispatches = 3
	shipErr := `{"class":"precondition","code":"REPO_CONTRACT_GATE","message":"` + strings.ReplaceAll(readC2035Round(t, "round3.json").GateFailures[0], `"`, `'`) + `"}`
	if err := os.WriteFile(filepath.Join(cs.WorkspacePath, "ship-error.json"), []byte(shipErr), 0o644); err != nil {
		t.Fatal(err)
	}
	cs.AuditFailReasons = nil

	next, _, _ := o.decideAfterAuditFail(cs)

	got := eventsWithCode(*events, CodeRepairHandoff)
	if next != PhaseTDD || len(got) != 1 || got[0].Fields["decision"] != fixability.DecisionWould || got[0].Fields["lane"] != "H1,M3" || got[0].Fields["handoff"] != "M1,M2" {
		t.Fatalf("next %s, signals %+v; want today's repair round and one would_stop with lane H1,M3", next, got)
	}
	if _, err := os.Stat(filepath.Join(cs.WorkspacePath, RepairHandoffFile)); !os.IsNotExist(err) {
		t.Errorf("shadow wrote %s (%v); it must change no flow", RepairHandoffFile, err)
	}
}

func TestDecideAfterAuditFail_ALaneFindingAloneEarnsTheRepairRoundAtEnforce(t *testing.T) {
	t.Parallel()
	o, events := handoffOrchestrator(config.StageEnforce)
	r := readC2035Round(t, "round3.json")
	r.Failure.Defects = r.Failure.Defects[:1]
	cs := CycleState{CycleID: 2035, WorkspacePath: c2035Workspace(t, r)}

	next, _, _ := o.decideAfterAuditFail(cs)

	if got := eventsWithCode(*events, CodeRepairHandoff); next != PhaseTDD || len(got) != 0 {
		t.Errorf("next %s, signals %+v; want a repair round and no signal without a hand-off finding", next, got)
	}
}

func TestDecideAfterAuditFail_OffChecksNothing(t *testing.T) {
	t.Parallel()
	o, events := handoffOrchestrator(config.StageOff)
	next, _, _ := o.decideAfterAuditFail(c2035State(t, "round1.json"))
	if next != PhaseTDD || len(eventsWithCode(*events, CodeRepairHandoff)) != 0 {
		t.Errorf("next %s, want today's repair round and no fixability signal at off", next)
	}
}

func TestRepairHandoff_AnUnwritableRecordWarnsAndStillStops(t *testing.T) {
	t.Parallel()
	o, _ := handoffOrchestrator(config.StageEnforce)
	var stderr strings.Builder
	o.stderr = &stderr
	cs := c2035State(t, "round1.json")
	if err := os.Mkdir(filepath.Join(cs.WorkspacePath, RepairHandoffFile), 0o755); err != nil {
		t.Fatal(err)
	}
	fb, _ := phasecontract.ReadFailureBlock(cs.WorkspacePath, string(PhaseAudit))

	_, stop := o.repairHandoff(cs, fb, retryEnvelope{Legal: []retryAction{retryActionRetryTDD}})

	if !stop || !strings.Contains(stderr.String(), RepairHandoffFile+" not written") {
		t.Errorf("stop %t, stderr %q; want the stop and a WARN", stop, stderr.String())
	}
}

func TestOpenConsoleIDs_AnUnreadableInboxNamesNoItem(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "inbox"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	o, _ := handoffOrchestrator(config.StageEnforce)
	var stderr strings.Builder
	o.stderr = &stderr
	if ids := o.openConsoleIDs(filepath.Join(evolveDir, "runs", "cycle-1")); ids != nil || !strings.Contains(stderr.String(), "WARN fixability: the inbox") {
		t.Errorf("ids = %v, stderr %q; want none and a WARN", ids, stderr.String())
	}
}

func TestResumeAfterAudit_AHandoffKeepsTheTerminationOfTheResumedCycle(t *testing.T) {
	t.Parallel()
	o, _ := handoffOrchestrator(config.StageEnforce)
	cs := c2035State(t, "round1.json")
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
	timings := []phaseTimingEntry{{Phase: "audit", Verdict: VerdictFAIL}}
	cr := &cycleRun{o: &Orchestrator{}, cs: CycleState{WorkspacePath: ws}}

	sealed := cr.sealHandedOff(&result, timings, repairHandoffReasonPrefix+"handoff_blocker: blockers M1 (red_test)")

	if !sealed || result.TerminationReason != CycleTerminationHandedOff {
		t.Fatalf("sealed %t, result %+v", sealed, result)
	}
	d := timings[0].Diagnostics
	if len(d) != 1 || d[0].Code != cyclestate.DiagCodeHandoffRoute || d[0].Subject != "retro-minting-files-through-inbox-add-lane" || !strings.HasPrefix(d[0].Message, "handed off: handoff_blocker") {
		t.Errorf("diagnostics = %+v, want one BLOCKER_ROUTE for the committed item", d)
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

func TestDecideAfterAuditFail_ANonBlockingHandoffFindingIsRecordedAndRepairs(t *testing.T) {
	t.Parallel()
	o, events := handoffOrchestrator(config.StageEnforce)
	r := readC2035Round(t, "round1.json")
	r.Failure.Defects = r.Failure.Defects[1:]
	cs := CycleState{CycleID: 2035, WorkspacePath: c2035Workspace(t, r)}

	next, _, _ := o.decideAfterAuditFail(cs)

	got := eventsWithCode(*events, CodeRepairHandoff)
	if next != PhaseTDD || len(got) != 1 || got[0].Fields["decision"] != fixability.DecisionNone || got[0].Fields["handoff"] != "M2" || got[0].Severity != signalcenter.SeverityInfo {
		t.Errorf("next %s, signals %+v; want a repair round and one INFO none naming M2", next, got)
	}
}

func TestDecideAfterAuditFail_NoCheckWhenThePolicyOffersNoRepairRound(t *testing.T) {
	t.Parallel()
	o, events := handoffOrchestrator(config.StageEnforce)
	cs := c2035State(t, "round1.json")
	cs.AuditRepairAttempts = 2

	next, reason, _ := o.decideAfterAuditFail(cs)

	if next != PhaseRetro || strings.HasPrefix(reason, repairHandoffReasonPrefix) || len(eventsWithCode(*events, CodeRepairHandoff)) != 0 {
		t.Errorf("next %s reason %q, want the budget decline and no fixability check", next, reason)
	}
}

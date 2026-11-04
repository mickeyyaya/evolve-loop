package core

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type auditIssue struct{ id, severity, title string }

func auditReportWith(class string, issues ...auditIssue) string {
	var b strings.Builder
	b.WriteString("# Audit\n\n## Verdict\nFAIL\n\n")
	b.WriteString(phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
		&phasecontract.FailureBlock{Class: class, Defects: []string{"EGPS: red_count=1 [x]"}}))
	b.WriteString("\n\n## Issues\n")
	for _, i := range issues {
		b.WriteString("### " + i.id + " (" + i.severity + ") — " + i.title + "\n")
	}
	return b.String()
}

func writeAuditRounds(t *testing.T, class string, archived [][]auditIssue, live []auditIssue) string {
	t.Helper()
	ws := t.TempDir()
	for i, issues := range archived {
		name := phasecontract.RoundArchiveFilename("audit-report.md", i+1)
		if err := os.WriteFile(filepath.Join(ws, name), []byte(auditReportWith(class, issues...)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(auditReportWith(class, live...)), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

var (
	alphaHigh = auditIssue{"H1", "HIGH", "the repair loop swallows the audit error"}
	betaMed   = auditIssue{"M1", "MEDIUM", "the brief omits the failing predicate"}
	gammaMed  = auditIssue{"M2", "MEDIUM", "the retry envelope ignores the attempt count"}
	deltaLow  = auditIssue{"L1", "LOW", "a log line names the wrong phase"}
)

func convergenceWorkflow(stage string, maxFixRounds int) policy.WorkflowConfig {
	wf := policy.Policy{}.WorkflowConfig()
	wf.Convergence.Stage = stage
	wf.Convergence.MaxFixRounds = maxFixRounds
	return wf
}

type convergenceRun struct {
	next    Phase
	reason  string
	signals []signalcenter.Event
}

func decideWith(t *testing.T, wf policy.WorkflowConfig, ws string, dispatches, attempts int) convergenceRun {
	t.Helper()
	center, events := captureSignals()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(wf), WithSignalCenter(center), WithStderr(&bytes.Buffer{}))
	next, reason, _ := o.decideAfterAuditFail(CycleState{CycleID: 2142, WorkspacePath: ws, AuditDispatches: dispatches, AuditRepairAttempts: attempts})
	return convergenceRun{next: next, reason: reason, signals: *events}
}

func (r convergenceRun) rungEvents() []signalcenter.Event {
	var got []signalcenter.Event
	for _, e := range r.signals {
		if e.Code == CodeConvergenceRung {
			got = append(got, e)
		}
	}
	return got
}

func (r convergenceRun) flowSignals() []signalcenter.Code {
	var got []signalcenter.Code
	for _, e := range r.signals {
		if e.Code != CodeConvergenceRung {
			got = append(got, e.Code)
		}
	}
	return got
}

func requireOneRung(t *testing.T, r convergenceRun, want map[string]string) signalcenter.Event {
	t.Helper()
	events := r.rungEvents()
	if len(events) != 1 {
		t.Fatalf("want exactly one %s signal, got %d: %+v", CodeConvergenceRung, len(events), r.signals)
	}
	e := events[0]
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("%s field %s = %q, want %q (fields %+v)", CodeConvergenceRung, k, e.Fields[k], v, e.Fields)
		}
	}
	if e.Cycle != 2142 || e.Phase != string(PhaseAudit) || e.Kind != signalcenter.KindPhaseOutcome {
		t.Errorf("%s event identity = cycle %d phase %q kind %q, want cycle 2142, phase audit, kind phase outcome", CodeConvergenceRung, e.Cycle, e.Phase, e.Kind)
	}
	return e
}

func twoRoundFixture(t *testing.T) string {
	t.Helper()
	return writeAuditRounds(t, "code-audit-fail", [][]auditIssue{{alphaHigh, betaMed, deltaLow}}, []auditIssue{alphaHigh, gammaMed})
}

func TestAuditRepairConvergence_PolicyCarriesTheConvergenceConfig(t *testing.T) {
	t.Parallel()

	got := policy.Policy{}.WorkflowConfig().Convergence
	if want := (policy.Policy{}).ConvergenceConfig(); !reflect.DeepEqual(got, want) {
		t.Fatalf("WorkflowConfig().Convergence = %+v, want the resolved default %+v", got, want)
	}
	if got.Stage != "shadow" {
		t.Errorf("default stage = %q, want shadow: the audit-repair consult ships recording first", got.Stage)
	}

	enforcing := policy.Policy{Workflow: &policy.WorkflowPolicy{Convergence: &policy.ConvergencePolicy{Stage: "enforce"}}}
	if stage := enforcing.WorkflowConfig().Convergence.Stage; stage != "enforce" {
		t.Errorf("policy stage enforce resolved to %q", stage)
	}
}

func TestAuditRepairConvergence_JudgmentsAreTheAuditRoundsInOrder(t *testing.T) {
	t.Parallel()
	ws := twoRoundFixture(t)

	rounds := auditRepairJudgments(ws, 2, &bytes.Buffer{})

	if len(rounds) != 2 || rounds[0].Index != 0 || rounds[1].Index != 1 {
		t.Fatalf("want J_0 (round-1 archive) and J_1 (live report), got %+v", rounds)
	}
	open := func(j convergence.Judgment) (n int) {
		for _, f := range j.Findings {
			if f.Status == convergence.StatusOpen {
				n++
			}
		}
		return n
	}
	if open(rounds[0]) != 3 || open(rounds[1]) != 2 {
		t.Errorf("open findings per round = %d, %d, want 3 then 2", open(rounds[0]), open(rounds[1]))
	}
	fixedIDs := map[string]bool{}
	for _, f := range rounds[1].Findings {
		if f.Status == convergence.StatusFixed {
			fixedIDs[f.ID] = true
		}
	}
	if len(fixedIDs) != 2 {
		t.Errorf("J_1 lists %d FIXED findings, want 2: the medium beta and the low delta are gone from the live report", len(fixedIDs))
	}
	idOf := func(j convergence.Judgment, sev convergence.Severity) string {
		for _, f := range j.Findings {
			if f.Severity == sev && f.Status == convergence.StatusOpen {
				return f.ID
			}
		}
		return ""
	}
	if a, b := idOf(rounds[0], convergence.SeverityHigh), idOf(rounds[1], convergence.SeverityHigh); a == "" || a != b {
		t.Errorf("the persistent HIGH keeps one id across rounds: %q vs %q", a, b)
	}
	for _, j := range rounds {
		for _, f := range j.Findings {
			if f.Class != "correctness" {
				t.Errorf("finding %s class = %q, want correctness: an audit defect blocks on the acceptance", f.ID, f.Class)
			}
			if f.Status == convergence.StatusDeferred || f.Status == convergence.StatusFiled {
				t.Errorf("finding %s status %s: the adapter never defers or files an audit finding", f.ID, f.Status)
			}
		}
	}
}

func TestAuditRepairConvergence_JudgmentsSkipAMissingArchiveAndRefuseAMissingLiveReport(t *testing.T) {
	t.Parallel()

	liveOnly := writeAuditRounds(t, "code-audit-fail", nil, []auditIssue{alphaHigh})
	if rounds := auditRepairJudgments(liveOnly, 3, &bytes.Buffer{}); len(rounds) != 1 || rounds[0].Index != 0 {
		t.Errorf("a live report with no archive is one judgment J_0, got %+v", rounds)
	}

	var warn bytes.Buffer
	if rounds := auditRepairJudgments(t.TempDir(), 2, &warn); len(rounds) != 0 {
		t.Errorf("no live report must give no judgment, got %+v", rounds)
	}
	if warn.Len() == 0 {
		t.Errorf("an absent live report must be reported on the warn writer, not skipped silently")
	}
}

func TestAuditRepairConvergence_InputIsTheAuditRepairLoop(t *testing.T) {
	t.Parallel()
	rounds := auditRepairJudgments(twoRoundFixture(t), 2, &bytes.Buffer{})
	cfg := policy.Policy{}.ConvergenceConfig()
	fixer := convergence.TierEffort{Family: "claude-tmux", Tier: "deep", Model: "opus", Effort: "high"}
	judge := convergence.TierEffort{Family: "claude-tmux", Tier: "balanced", Model: "sonnet", Effort: "medium"}
	headroom := convergence.HeadroomTable{"claude-tmux": {"deep": {Model: "opus", Effort: "high"}}}

	in := auditRepairConvergenceInput(cfg, rounds, fixer, judge, headroom)

	if in.Loop != convergence.LoopAuditRepair || in.Round != 1 || len(in.Rounds) != 2 {
		t.Fatalf("input loop %q round %d rounds %d, want audit-repair, 1, 2", in.Loop, in.Round, len(in.Rounds))
	}
	if in.Fixer != fixer || in.Judge != judge || !reflect.DeepEqual(in.Headroom, headroom) || !reflect.DeepEqual(in.Config, cfg) {
		t.Errorf("input must carry fixer, judge, headroom and config unchanged: %+v", in)
	}
	if err := in.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestAuditRepairConvergence_ReplayWithABudgetOfTwoRaisesTheJudgeAndFreshensTheFixer(t *testing.T) {
	t.Parallel()
	cfg := policy.Policy{}.ConvergenceConfig()
	cfg.MaxFixRounds = 2
	deep := convergence.TierEffort{Family: "claude-tmux", Tier: "deep", Model: "opus", Effort: "high"}
	top := convergence.TierEffort{Family: "claude-tmux", Tier: "top", Model: "opus", Effort: "xhigh"}
	headroom := convergence.HeadroomTable{"claude-tmux": {
		"fast": {Model: "haiku", Effort: "low"}, "balanced": {Model: "sonnet", Effort: "medium"},
		"deep": {Model: "opus", Effort: "high"}, "top": {Model: "opus", Effort: "xhigh"},
	}}
	roundOne, roundTwo := []auditIssue{alphaHigh, betaMed, deltaLow}, []auditIssue{alphaHigh, gammaMed}
	twoRounds := writeAuditRounds(t, "code-audit-fail", [][]auditIssue{roundOne}, roundTwo)
	threeRounds := writeAuditRounds(t, "code-audit-fail", [][]auditIssue{roundOne, roundTwo}, []auditIssue{alphaHigh})

	decideAt := func(ws string, dispatches int) convergence.Decision {
		rounds := auditRepairJudgments(ws, dispatches, &bytes.Buffer{})
		in := auditRepairConvergenceInput(cfg, rounds, deep, deep, headroom)
		if err := in.Validate(); err != nil {
			t.Fatalf("Validate at %d dispatches: %v", dispatches, err)
		}
		return convergence.Decide(in)
	}

	if rounds := auditRepairJudgments(threeRounds, 3, &bytes.Buffer{}); len(rounds) != 3 {
		t.Fatalf("three audit rounds on disk must give three judgments, got %d", len(rounds))
	}

	final := decideAt(twoRounds, 2)
	if final.Action != convergence.ActionContinue || final.Rung != 2 || !final.FreshContext || final.JudgeRaise != top {
		t.Errorf("round 2 of 2: %+v, want a Continue at rung 2 with the fresh-context fixer and the judge raised to top xhigh", final)
	}
	if len(final.Defer) != 0 || len(final.File) != 0 {
		t.Errorf("round 2 of 2 defers %q and files %q, want an audit finding never deferred or filed", final.Defer, final.File)
	}

	stop := decideAt(threeRounds, 3)
	if stop.Action != convergence.ActionStop || len(stop.Defer) != 0 {
		t.Errorf("after the budget: %+v, want a Stop with nothing deferred", stop)
	}
}

func TestAuditRepairConvergence_ShadowRecordsTheRungAndLeavesTheFlowAlone(t *testing.T) {
	t.Parallel()
	ws := twoRoundFixture(t)

	stopsConvergence := decideWith(t, convergenceWorkflow("shadow", 1), ws, 2, 1)
	continuesConvergence := decideWith(t, convergenceWorkflow("shadow", 3), ws, 2, 1)

	if stopsConvergence.next != PhaseTDD || !strings.HasPrefix(stopsConvergence.reason, auditRepairReasonPrefix) {
		t.Fatalf("shadow must keep the envelope's grant: next %s reason %q", stopsConvergence.next, stopsConvergence.reason)
	}
	if stopsConvergence.next != continuesConvergence.next || stopsConvergence.reason != continuesConvergence.reason {
		t.Errorf("shadow flow depends on the decision: Stop run (%s, %q) vs Continue run (%s, %q)",
			stopsConvergence.next, stopsConvergence.reason, continuesConvergence.next, continuesConvergence.reason)
	}
	if !reflect.DeepEqual(stopsConvergence.flowSignals(), []signalcenter.Code{CodeAuditRepairGranted}) {
		t.Errorf("shadow must emit only the unchanged repair signal besides the rung: %v", stopsConvergence.flowSignals())
	}
	e := requireOneRung(t, stopsConvergence, map[string]string{"stage": "shadow", "loop": "audit-repair", "round": "1", "action": "Stop", "rung": "3"})
	if e.Severity != signalcenter.SeverityInfo {
		t.Errorf("a recorded decision is INFO, got %s", e.Severity)
	}
	requireOneRung(t, continuesConvergence, map[string]string{"stage": "shadow", "action": "Continue", "rung": "2", "fresh_context": "true"})
}

func TestAuditRepairConvergence_ShadowRecordsWhenTheEnvelopeAlreadyDeclines(t *testing.T) {
	t.Parallel()
	ws := writeAuditRounds(t, "superseded-predicate-contradiction", nil, []auditIssue{alphaHigh})

	r := decideWith(t, convergenceWorkflow("shadow", 3), ws, 1, 0)

	if r.next != PhaseRetro || !reflect.DeepEqual(r.flowSignals(), []signalcenter.Code{CodeAuditRepairDeclined}) {
		t.Fatalf("the declined envelope is unchanged: next %s signals %v", r.next, r.flowSignals())
	}
	requireOneRung(t, r, map[string]string{"stage": "shadow", "round": "0", "action": "Continue", "rung": "0"})
}

func TestAuditRepairConvergence_EnforceStopEndsTheRepairTheEnvelopeWouldGrant(t *testing.T) {
	t.Parallel()
	ws := twoRoundFixture(t)

	r := decideWith(t, convergenceWorkflow("enforce", 1), ws, 2, 1)

	if r.next != PhaseRetro || !strings.HasPrefix(r.reason, auditDeclineReasonPrefix) {
		t.Fatalf("enforce Stop must send the cycle to retro with a decline reason: next %s reason %q", r.next, r.reason)
	}
	if !strings.Contains(strings.ToLower(r.reason), "convergence") {
		t.Errorf("the decline reason must name the convergence decision: %q", r.reason)
	}
	if !reflect.DeepEqual(r.flowSignals(), []signalcenter.Code{CodeAuditRepairDeclined}) {
		t.Errorf("the enforced Stop emits the decline signal, not a grant: %v", r.flowSignals())
	}
	requireOneRung(t, r, map[string]string{"stage": "enforce", "action": "Stop", "rung": "3"})
}

func TestAuditRepairConvergence_EnforceContinueGrantsTheRepairTheEnvelopeGrants(t *testing.T) {
	t.Parallel()
	ws := twoRoundFixture(t)

	r := decideWith(t, convergenceWorkflow("enforce", 3), ws, 2, 1)

	if r.next != PhaseTDD || !strings.HasPrefix(r.reason, auditRepairReasonPrefix) {
		t.Fatalf("enforce Continue keeps the repair round: next %s reason %q", r.next, r.reason)
	}
	if !reflect.DeepEqual(r.flowSignals(), []signalcenter.Code{CodeAuditRepairGranted}) {
		t.Errorf("signals %v, want the repair grant", r.flowSignals())
	}
	requireOneRung(t, r, map[string]string{"stage": "enforce", "action": "Continue", "rung": "2", "fresh_context": "true"})
}

func TestAuditRepairConvergence_EnforceNeverGrantsWhatTheEnvelopeDeclines(t *testing.T) {
	t.Parallel()
	ws := writeAuditRounds(t, "superseded-predicate-contradiction", nil, []auditIssue{alphaHigh})

	r := decideWith(t, convergenceWorkflow("enforce", 3), ws, 1, 0)

	if r.next != PhaseRetro || !strings.HasPrefix(r.reason, auditDeclineReasonPrefix) {
		t.Errorf("a Continue decision cannot lift an envelope decline: next %s reason %q", r.next, r.reason)
	}
}

func TestAuditRepairConvergence_AnUnresolvedConfigConsultsNothing(t *testing.T) {
	t.Parallel()
	ws := twoRoundFixture(t)
	bare := policy.WorkflowConfig{}

	r := decideWith(t, bare, ws, 2, 1)

	if len(r.rungEvents()) != 0 {
		t.Errorf("a workflow config with no convergence block must not record a rung: %+v", r.rungEvents())
	}
	if r.next != PhaseTDD || !strings.HasPrefix(r.reason, auditRepairReasonPrefix) {
		t.Errorf("the flow with no convergence block is unchanged: next %s reason %q", r.next, r.reason)
	}
}

func TestAuditRepairConvergence_ARepairWithNoReadableRoundsConsultsNothing(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	r := decideWith(t, convergenceWorkflow("enforce", 1), ws, 1, 0)

	if len(r.rungEvents()) != 0 {
		t.Errorf("no audit report gives no judgment, so no rung: %+v", r.rungEvents())
	}
}

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func spendRun(t *testing.T, stage config.Stage, current Phase) (*cycleRun, *[]signalcenter.Event) {
	t.Helper()
	ws := t.TempDir()
	ledger, err := os.ReadFile(filepath.Join("..", "cyclespend", "testdata", "cycle-2035", llmcalls.Filename))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{llmcalls.Filename: string(ledger), "triage-decision.json": `{"top_n":[{"id":"retro-minting-files-through-inbox-add-lane"}]}`}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wf := policy.Policy{}.WorkflowConfig()
	wf.CycleSpend.Stage = stage
	center, events := captureSignals()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(wf), WithSignalCenter(center))
	cr := &cycleRun{o: o, cycle: 2035, current: current, cs: CycleState{CycleID: 2035, WorkspacePath: ws, AuditDispatches: 4},
		phaseTimings: []phaseTimingEntry{{Phase: string(current), Verdict: VerdictPASS}}}
	return cr, events
}

func TestCheckCycleSpend_Cycle2035AtEnforceStopsAndHandsOff(t *testing.T) {
	t.Parallel()
	cr, events := spendRun(t, config.StageEnforce, PhaseBuild)
	cr.cs.ActiveWorktree = "/dev/cl-c2035-lane"
	cr.checkCycleSpend()
	if cr.scheduledNext != "" {
		t.Fatal("build has no legal edge to retro, so the stop waits for the audit boundary")
	}
	cr.current, cr.lastVerdict = PhaseAudit, VerdictPASS
	cr.checkCycleSpend()
	if cr.scheduledNext != "" {
		t.Fatal("a passing audit ships: the spend is already spent")
	}
	cr.lastVerdict = VerdictFAIL

	cr.checkCycleSpend()

	got := eventsWithCode(*events, CodeCycleSpendLimit)
	if cr.scheduledNext != PhaseRetro || cr.result.TerminationReason != CycleTerminationHandedOff {
		t.Fatalf("next %q termination %q, want a stop to retro and handed-off", cr.scheduledNext, cr.result.TerminationReason)
	}
	if len(got) != 2 || got[0].Fields["decision"] != cyclespend.ActionOver || got[0].Fields["phase"] != "build" || got[1].Fields["decision"] != cyclespend.ActionStop || got[1].Fields["phase"] != "audit" ||
		got[1].Fields["limit"] != cyclespend.LimitLLMMinutes || got[1].Fields["value"] != "160.1" || got[1].Fields["max"] != "90" || got[1].Fields["partial"] != "false" {
		t.Errorf("signals = %+v, want over at build, then stop at the failed audit on max_llm_minutes at 160.1 of 90", got)
	}
	d := cr.phaseTimings[0].Diagnostics
	if len(d) != 1 || d[0].Code != cyclestate.DiagCodeHandoffRoute || !strings.Contains(d[0].Message, "spend_limit: max_llm_minutes 160.1 of 90; salvage /dev/cl-c2035-lane") {
		t.Errorf("diagnostics = %+v, want the hand-off route with the spend limit and the salvage pointer", d)
	}
	cr.checkCycleSpend()
	if len(eventsWithCode(*events, CodeCycleSpendLimit)) != 2 {
		t.Error("a stopped cycle is not checked again")
	}
}

func TestCheckCycleSpend_AFixabilityHandoffOnTheSameAuditKeepsOneRoute(t *testing.T) {
	t.Parallel()
	cr, events := spendRun(t, config.StageEnforce, PhaseAudit)
	cr.lastVerdict = VerdictFAIL
	cr.result.TerminationReason = CycleTerminationHandedOff
	cr.phaseTimings[0].Diagnostics = []Diagnostic{{Code: cyclestate.DiagCodeHandoffRoute, Message: "handed off: handoff_blocker"}}

	cr.checkCycleSpend()

	if len(cr.phaseTimings[0].Diagnostics) != 1 || cr.scheduledNext != PhaseRetro || len(eventsWithCode(*events, CodeCycleSpendLimit)) != 1 {
		t.Errorf("diagnostics %+v next %q; want the one fixability route, retro and the stop signal", cr.phaseTimings[0].Diagnostics, cr.scheduledNext)
	}
}

func TestCheckCycleSpend_ShadowRecordsWouldStopOnceAndChangesNoFlow(t *testing.T) {
	t.Parallel()
	cr, events := spendRun(t, config.StageShadow, PhaseAudit)

	cr.lastVerdict = VerdictFAIL
	cr.checkCycleSpend()
	cr.checkCycleSpend()

	got := eventsWithCode(*events, CodeCycleSpendLimit)
	if cr.scheduledNext != "" || cr.result.TerminationReason != "" || len(got) != 1 || got[0].Fields["decision"] != cyclespend.ActionWouldStop {
		t.Errorf("shadow: next %q, signals %+v; want one would_stop and no flow change", cr.scheduledNext, got)
	}
}

func TestCheckCycleSpend_NoCheckOffOrUnderTheLimitOrAtTheEnd(t *testing.T) {
	t.Parallel()
	off, offEvents := spendRun(t, config.StageOff, PhaseBuild)
	under, underEvents := spendRun(t, config.StageEnforce, PhaseBuild)
	under.o.workflowConfig.CycleSpend.Limits = cyclespend.Limits{MaxLLMMinutes: 600, MaxFreshTokens: 20_000_000, MaxCacheReadTokens: 500_000_000, MaxAuditRounds: 4}
	retro, retroEvents := spendRun(t, config.StageEnforce, PhaseRetro)

	for _, cr := range []*cycleRun{off, under, retro} {
		cr.lastVerdict = VerdictFAIL
		cr.checkCycleSpend()
		if cr.scheduledNext != "" {
			t.Errorf("phase %s: scheduled %q, want no stop when off, under the limits, or at retro", cr.current, cr.scheduledNext)
		}
	}
	if len(*offEvents)+len(*underEvents)+len(*retroEvents) != 0 {
		t.Errorf("signals = %d %d %d, want none", len(*offEvents), len(*underEvents), len(*retroEvents))
	}
}

func TestCheckCycleSpend_AnUnreadableLedgerWarnsAndChecksTheRest(t *testing.T) {
	t.Parallel()
	cr, events := spendRun(t, config.StageShadow, PhaseBuild)
	var stderr strings.Builder
	cr.o.stderr = &stderr
	path := filepath.Join(cr.cs.WorkspacePath, llmcalls.Filename)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	cr.checkCycleSpend()

	got := eventsWithCode(*events, CodeCycleSpendLimit)
	if !strings.Contains(stderr.String(), "WARN cycle spend") || len(got) != 1 || got[0].Fields["partial"] != "true" || got[0].Fields["limit"] != cyclespend.LimitAuditRounds {
		t.Errorf("stderr = %q, signals %+v; want a WARN and a partial judgement on the audit rounds", stderr.String(), got)
	}
}

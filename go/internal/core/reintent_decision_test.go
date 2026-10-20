package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const cycle1953FailureDecision = `{
  "category": "intent-malformed",
  "level": "task",
  "evidence": "failure-dossier.json audit_declared class=intent-malformed level=task; lane item files=[docs/operations/runtime-reference.md] while both acceptance criteria need go/internal/inboxmover/lifecycle/item.go:189 changed",
  "justification": "the task intent is undeliverable in a lane until the console part lands",
  "action": "defer-or-quarantine",
  "fix_type": "reintent",
  "schema_version": 1
}`

func cycle1953Workspace(t *testing.T, class string, withDecision bool) string {
	t.Helper()
	ws := t.TempDir()
	writeAuditWithFailure(t, ws, "FAIL", class,
		"H1: the lane part's acceptance needs the protected writer go/internal/inboxmover/lifecycle/item.go:189 changed; round 3 repeats the round-2 diff")
	if withDecision {
		if err := os.WriteFile(filepath.Join(ws, "failure-decision.json"), []byte(cycle1953FailureDecision), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

func declineThenRetro(t *testing.T, o *Orchestrator, cs CycleState) CycleState {
	t.Helper()
	next, reason, sig := o.decideAfterAuditFail(cs)
	if next != PhaseRetro || sig != nil {
		t.Fatalf("precondition: the audit FAIL must decline to retro, got next=%s sig=%+v reason=%q", next, sig, reason)
	}
	consumeAuditRepairGrant(&cs, reason)
	cs.CompletedPhases = append(cs.CompletedPhases, string(PhaseAudit), string(PhaseRetro))
	return cs
}

func routingDecisionAt(t *testing.T, ws string, seq string) router.RouterDecision {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ws, "routing-decision-"+seq+".json"))
	if err != nil {
		t.Fatalf("the terminal decision must be recorded as routing-decision-%s.json: %v", seq, err)
	}
	var d router.RouterDecision
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDecideAfterRetroRouted_Cycle1953ReintentDecisionEndsTheCycle(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.ledger = led
	ws := cycle1953Workspace(t, "intent-malformed", true)
	cs := declineThenRetro(t, o, CycleState{CycleID: 1953, WorkspacePath: ws, AuditRepairAttempts: 1, AuditDispatches: 2})

	next, _, reason, sig := o.decideAfterRetroRouted(context.Background(), 1953, cs, 12, VerdictPASS, nil, router.RouteInput{})

	if sig != nil {
		t.Fatalf("a task-level reintent is no system failure: %+v", sig)
	}
	if next != PhaseEnd {
		t.Fatalf("next = %s, want end: a defer-or-quarantine decision must not dispatch another tdd/build round (cycle 1953 rebuilt for 23 min)", next)
	}
	if !strings.HasPrefix(reason, auditDeferReasonPrefix) || !strings.Contains(reason, "intent-malformed") || !strings.Contains(reason, "reintent") {
		t.Errorf("reason = %q, want the %q prefix naming the category and the fix type", reason, auditDeferReasonPrefix)
	}
	d := routingDecisionAt(t, ws, "12")
	if d.NextPhase != router.PhaseEnd || d.Reason != reason {
		t.Errorf("recorded decision = %+v, want next=end with the defer reason", d)
	}
	if d.Evidence["category"] != "intent-malformed" || d.Evidence["fix_type"] != "reintent" || d.Evidence["source"] != "audit" {
		t.Errorf("recorded evidence = %+v, want the audit's category and fix type", d.Evidence)
	}
	var rows int
	for _, e := range led.entries {
		if e.Kind == "routing_decision" && e.ArtifactSHA256 != "" {
			rows++
		}
	}
	if rows != 1 {
		t.Errorf("routing_decision ledger rows = %d, want 1 hash-bound row recording the defer", rows)
	}
}

func TestDecideAfterRetro_ADeferDecisionOutranksTheBookkeepingRegrade(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(nil)
	ws := cycle1953Workspace(t, "intent-malformed", false)
	cs := declineThenRetro(t, o, CycleState{CycleID: 1953, WorkspacePath: ws})
	cs.AuditFailReasons = []string{tConflictPASS, tLedger}

	next, _, reason, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig != nil || next != PhaseEnd || !strings.HasPrefix(reason, auditDeferReasonPrefix) {
		t.Fatalf("next=%s reason=%q sig=%+v, want end by the defer: a re-audit of the identical diff must not follow a reintent decision", next, reason, sig)
	}
}

func TestDecideAfterRetro_TheRetroFailureDecisionAloneDefersAfterADecline(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	ws := cycle1953Workspace(t, "code-audit-fail", true)
	cs := declineThenRetro(t, o, CycleState{CycleID: 1953, WorkspacePath: ws, AuditRepairAttempts: 2})

	next, _, reason, _ := o.decideAfterRetroRouted(context.Background(), 1953, cs, 1, VerdictPASS, nil, router.RouteInput{})

	if next != PhaseEnd || !strings.HasPrefix(reason, auditDeferReasonPrefix) {
		t.Fatalf("next=%s reason=%q, want end: failure-decision.json says defer-or-quarantine", next, reason)
	}
	if d := routingDecisionAt(t, ws, "1"); d.Evidence["source"] != "failure-decision.json" {
		t.Errorf("evidence = %+v, want the retro's failure decision named as the source", d.Evidence)
	}
}

func TestDecideAfterRetroRouted_ADeclineWithoutADeferDecisionKeepsTheRetroRoute(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	ws := cycle1953Workspace(t, "superseded-predicate-contradiction", false)
	cs := declineThenRetro(t, o, CycleState{CycleID: 1684, WorkspacePath: ws})

	next, _, reason, _ := o.decideAfterRetroRouted(context.Background(), 1684, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if next != PhaseTDD || strings.HasPrefix(reason, auditDeferReasonPrefix) {
		t.Fatalf("next=%s reason=%q, want the retro-routed tdd round an unrecognised class still earns", next, reason)
	}
}

func TestAuditDeclineDefers_NeedsADecline(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(nil)
	ws := cycle1953Workspace(t, "intent-malformed", true)

	if d, ok := o.auditDeclineDefers(CycleState{WorkspacePath: ws}); ok {
		t.Fatalf("no audit decline is on record, so nothing defers; got %+v", d)
	}
	if d, ok := o.auditDeclineDefers(CycleState{WorkspacePath: t.TempDir(), AuditDeclineReason: "audit declared no failure class; nothing to base a retry on"}); ok {
		t.Fatalf("a decline with no defer evidence must not defer; got %+v", d)
	}
}

func TestRecordAndBranch_Cycle1953ReintentDecisionBreaksTheLoop(t *testing.T) {
	t.Parallel()
	cr := retroGateHarness(t, phasespec.Catalog{})
	cr.o.strategy = fixedNextStrategy{next: "tdd"}
	cr.o.cfg = config.RoutingConfig{Stage: config.StageAdvisory}
	cr.cs = declineThenRetro(t, cr.o, CycleState{CycleID: 1953, WorkspacePath: cycle1953Workspace(t, "intent-malformed", true), AuditRepairAttempts: 1})
	cr.cycle = 1953

	act, err := cr.recordAndBranch(PhaseRetro, dispatchResult{resp: PhaseResponse{Verdict: VerdictPASS}, attemptCount: 1})

	if err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}
	if act != loopBreak || cr.scheduledNext == PhaseTDD || cr.scheduledNext == PhaseBuild {
		t.Fatalf("act=%v scheduled=%s, want the loop to break with no tdd/build round", act, cr.scheduledNext)
	}
	if !strings.Contains(cr.result.RetroDecision, auditDeferReasonPrefix) {
		t.Errorf("RetroDecision = %q, want the defer reason", cr.result.RetroDecision)
	}
}

func TestDecideAfterRetroRouted_AFloorHaltOutranksADeferDecision(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	ws := cycle1953Workspace(t, "infra-systemic", true)
	cs := CycleState{CycleID: 1953, WorkspacePath: ws, AuditDeclineReason: "declared class infra-systemic is system-level"}

	next, _, reason, sig := o.decideAfterRetroRouted(context.Background(), 1953, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if sig == nil || !sig.Halt || next != PhaseEnd || strings.HasPrefix(reason, auditDeferReasonPrefix) {
		t.Fatalf("next=%s reason=%q sig=%+v, want the floor halt to win over the defer", next, reason, sig)
	}
}

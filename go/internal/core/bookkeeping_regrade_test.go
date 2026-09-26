package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const (
	tConflictPASS = "verdict-conflict: auditor narrative=PASS but 2 deterministic gate(s) forced FAIL [defect-ledger, closure-claim] — the gate outranks the narrative (ship policy unchanged); both readings are recorded so the disagreement is weighable."
	tConflictWARN = "verdict-conflict: auditor narrative=WARN but 1 deterministic gate(s) forced FAIL [defect-ledger] — the gate outranks the narrative."
	tLedger       = "defect ledger: disposition-preflight: MISSING — this workspace holds no defect-dispositions.json at all, so 0 of 3 defect(s) inherited from cycle-1421 are dispositioned."
	tClosure      = `closure claim without a citation: "fixed the 1424 CRITICAL" — a report may not assert a prior cycle's defect is closed without naming the per-defect record on the same line.`
	tEGPS         = "EGPS verdict FAIL: red_count=1"
)

func TestBookkeepingRegradeEligible_Matrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		reasons []string
		want    bool
	}{
		{"conflict+ledger", []string{tConflictPASS, tLedger}, true},
		{"conflict+closure", []string{tConflictPASS, tClosure}, true},
		{"warn-narrative+both", []string{tConflictWARN, tLedger, tClosure}, true},
		{"ledger-only-no-conflict", []string{tLedger}, false},
		{"conflict-only", []string{tConflictPASS}, false},
		{"conflict+ledger+egps", []string{tConflictPASS, tLedger, tEGPS}, false},
		{"empty", nil, false},
		// Producers never emit a narrative=FAIL conflict line, but the matcher must reject a smuggled one.
		{"forged-fail-narrative", []string{"verdict-conflict: auditor narrative=FAIL but 1 deterministic gate(s) forced FAIL [defect-ledger]", tLedger}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BookkeepingRegradeEligible(tc.reasons); got != tc.want {
				t.Errorf("BookkeepingRegradeEligible(%v) = %v, want %v", tc.reasons, got, tc.want)
			}
		})
	}
}

func TestDecideAfterRetro_BookkeepingRegradeBranch(t *testing.T) {
	o := floorOrchestrator(nil)
	cs := CycleState{CycleID: 1430, WorkspacePath: t.TempDir(),
		AuditFailReasons: []string{tConflictPASS, tLedger}}

	next, _, reason, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)
	if sig != nil {
		t.Fatalf("unexpected system-failure signal: %+v", sig)
	}
	if next != PhaseAudit {
		t.Fatalf("next = %s, want audit (same-cycle bookkeeping re-grade)", next)
	}
	if !strings.HasPrefix(reason, BookkeepingRegradeReasonPrefix) {
		t.Errorf("reason = %q, want prefix %q (the RetroDecision contract string)", reason, BookkeepingRegradeReasonPrefix)
	}
	if !o.sm.CanTransition(PhaseRetro, PhaseAudit) {
		t.Error("retro→audit not SM-legal — cyclerun_record would loopAbort on the scheduled branch")
	}
}

func TestDecideAfterRetro_RegradeOncePerCycle(t *testing.T) {
	o := floorOrchestrator(nil)
	cs := CycleState{CycleID: 1430, WorkspacePath: t.TempDir(),
		AuditFailReasons:            []string{tConflictPASS, tLedger},
		BookkeepingRegradeAttempted: true}

	next, _, reason, _ := o.decideAfterRetro(cs, VerdictFAIL, nil)
	if next == PhaseAudit || strings.HasPrefix(reason, BookkeepingRegradeReasonPrefix) {
		t.Fatalf("second regrade granted (next=%s reason=%q) — the micro-cycle must be once per cycle", next, reason)
	}
}

func TestDecideAfterRetroRouted_RegradeNotRouterOverridable(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	cs := CycleState{CycleID: 1430, WorkspacePath: t.TempDir(),
		AuditFailReasons: []string{tConflictWARN, tClosure}}

	next, _, reason, sig := o.decideAfterRetroRouted(context.Background(), 1430, cs, 1, VerdictFAIL, nil, router.RouteInput{})
	if sig != nil {
		t.Fatalf("unexpected system-failure signal: %+v", sig)
	}
	if next != PhaseAudit {
		t.Fatalf("routed next = %s, want audit — the router overrode the deterministic micro-recovery", next)
	}
	if !strings.HasPrefix(reason, BookkeepingRegradeReasonPrefix) {
		t.Errorf("routed reason = %q, want the regrade contract prefix", reason)
	}
}

func TestRecordAndBranch_RegradeGrantConsumesSlotAndSchedulesAudit(t *testing.T) {
	t.Parallel()
	cr := retroGateHarness(t, phasespec.Catalog{})
	cr.cs.AuditFailReasons = []string{tConflictPASS, tLedger}
	dr := dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, attemptCount: 1}

	act, err := cr.recordAndBranch(PhaseRetro, dr)
	if err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}
	if act != loopNext || cr.scheduledNext != PhaseAudit {
		t.Fatalf("act=%v scheduledNext=%s, want loopNext + audit", act, cr.scheduledNext)
	}
	if !strings.Contains(cr.result.RetroDecision, BookkeepingRegradeReasonPrefix) {
		t.Errorf("RetroDecision = %q, want the regrade contract string", cr.result.RetroDecision)
	}
	if !cr.cs.BookkeepingRegradeAttempted {
		t.Fatal("grant did not consume the once-per-cycle slot — retro→audit would loop forever")
	}

	cr.current = PhaseRetro
	if _, err := cr.recordAndBranch(PhaseRetro, dr); err != nil {
		t.Fatalf("recordAndBranch (second): %v", err)
	}
	if cr.scheduledNext == PhaseAudit && strings.Contains(cr.result.RetroDecision, BookkeepingRegradeReasonPrefix) {
		t.Fatal("second FAIL re-granted the regrade — the bound is unwired")
	}
}

func TestDecideAfterRetro_FloorOutranksRegrade(t *testing.T) {
	o := floorOrchestrator(nil)
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS") // green artifacts + recorded FAIL = incoherent
	cs := CycleState{CycleID: 1430, WorkspacePath: dir}
	// No AuditFailReasons: an unexplained FAIL with green artifacts is the forged-verdict floor signature.

	next, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)
	if sig == nil || next != PhaseEnd {
		t.Fatalf("next=%s sig=%v — the verdict-incoherence floor must halt before any regrade consideration", next, sig)
	}
}

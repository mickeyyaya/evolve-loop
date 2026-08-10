package core

// bookkeeping_regrade_test.go — RED contract for the bookkeeping-regrade
// micro-cycle (inbox 0.92, three-perspective investigation 2026-08-10).
//
// The disease: cycles 1390-1429 show 6 FAILs where the auditor graded the work
// PASS/WARN and only deterministic bookkeeping gates (continuation-disposition
// preflight, closure-claim citations) forced FAIL. Each burned a full
// continuation re-drive (~2M tokens, measured 0/11 continuation pass rate)
// to author one JSON artifact. The fix: at the retro chokepoint, a FAIL whose
// ONLY explanations are bookkeeping-class routes to a bounded same-cycle audit
// re-dispatch (retro→audit, once per cycle) instead of dying to a continuation.
//
// Trust boundary: eligibility reads CycleState.AuditFailReasons (orchestrator
// memory, the ADR-0072 pattern) — never a workspace file an agent could author.
// Bound: CycleState.BookkeepingRegradeAttempted, also orchestrator-owned.

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
		// The auditor itself said FAIL (no conflict line): real defects — not ours.
		{"ledger-only-no-conflict", []string{tLedger}, false},
		// A conflict line alone names no bookkeeping gate reason to repair.
		{"conflict-only", []string{tConflictPASS}, false},
		// ANY non-bookkeeping reason (EGPS red, vet, tier…) disqualifies.
		{"conflict+ledger+egps", []string{tConflictPASS, tLedger, tEGPS}, false},
		{"empty", nil, false},
		// A narrative=FAIL conflict line can't occur (producer guards), but the
		// matcher must not accept one that an attacker smuggles into a message.
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

// The deterministic branch: eligible reasons + not-yet-attempted → retro→audit
// with the contract reason prefix; the audit edge must be SM-legal.
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

// Bounded: an already-attempted cycle falls through to the normal adapter path
// (no infinite retro→audit loop when the re-audit fails again).
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

// The routed path must treat the regrade like the floor: decided ABOVE the
// router, non-overridable — a strategy proposing tdd/end cannot eat it.
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

// Consumption WIRING pin (diff-review MEDIUM): a grant driven through the REAL
// recordAndBranch retro branch must consume the once-per-cycle slot and
// schedule audit; the immediate second FAIL must fall through — deleting the
// consumeBookkeepingRegradeGrant call would red this, not just the hand-set
// unit test above. (Resume-surface parity is the same single-source primitive,
// called at resume.go's history branch — the recordFloorVerdictFailure
// precedent for record/resume duplication.)
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

	// Second bookkeeping-only FAIL in the same cycle: no second grant.
	cr.current = PhaseRetro
	if _, err := cr.recordAndBranch(PhaseRetro, dr); err != nil {
		t.Fatalf("recordAndBranch (second): %v", err)
	}
	if cr.scheduledNext == PhaseAudit && strings.Contains(cr.result.RetroDecision, BookkeepingRegradeReasonPrefix) {
		t.Fatal("second FAIL re-granted the regrade — the bound is unwired")
	}
}

// Floor supremacy: a floor-category classification still halts an otherwise
// regrade-eligible cycle — the regrade sits BELOW the ADR-0072 floor.
func TestDecideAfterRetro_FloorOutranksRegrade(t *testing.T) {
	o := floorOrchestrator(nil)
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS") // green artifacts + recorded FAIL = incoherent
	cs := CycleState{CycleID: 1430, WorkspacePath: dir}
	// Deliberately NO AuditFailReasons: an unexplained FAIL with green artifacts
	// is the forged-verdict floor signature; regrade must not touch it.

	next, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)
	if sig == nil || next != PhaseEnd {
		t.Fatalf("next=%s sig=%v — the verdict-incoherence floor must halt before any regrade consideration", next, sig)
	}
}
